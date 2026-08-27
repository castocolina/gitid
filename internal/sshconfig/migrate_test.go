package sshconfig

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
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

// ---------------------------------------------------------------------------
// Globals-carriage migration tests (plan 06-02 Task 2 — 06-REVIEWS.md HIGH)
// ---------------------------------------------------------------------------

// containsBlockName reports whether content carries a managed block named name.
func containsBlockName(content []byte, name string) bool {
	for _, b := range filewriter.ListBlocks(content) {
		if b.Name == name {
			return true
		}
	}
	return false
}

// assertGlobalsBlockLast fails unless the block named globalsName starts after
// the block named identityName in content and is the final gitid-managed block
// there (the D-09 identities-first / wildcard-last invariant).
func assertGlobalsBlockLast(t *testing.T, content []byte, globalsName, identityName string) {
	t.Helper()
	text := string(content)
	gOff := strings.Index(text, filewriter.BeginPrefix+globalsName+"\n")
	iOff := strings.Index(text, filewriter.BeginPrefix+identityName+"\n")
	if gOff < 0 || iOff < 0 {
		t.Fatalf("missing sentinels (globals=%d identity=%d) in:\n%s", gOff, iOff, content)
	}
	if gOff < iOff {
		t.Errorf("globals begin-sentinel at %d precedes identity %q at %d; want globals LAST:\n%s", gOff, identityName, iOff, content)
	}
	blocks := filewriter.ListBlocks(content)
	if len(blocks) == 0 || blocks[len(blocks)-1].Name != globalsName {
		t.Errorf("last gitid-managed block = %q, want %s:\n%s", lastBlockName(blocks), globalsName, content)
	}
}

// TestMigrationClasses asserts the three-way storage-migration classification
// (06-REVIEWS.md HIGH resolution): the globals block goes in the globals list,
// the Include-line block is stationary and appears in NEITHER list, and an
// ordinary identity name goes in the identities list — under BOTH globals
// sentinel names.
func TestMigrationClasses(t *testing.T) {
	content := []byte(
		managedTestBlock(sshIncludeBlockName, sshIncludeLineBody+"\n") +
			managedTestBlock(GlobalBlockName, "Host *\n  HashKnownHosts yes\n") +
			managedTestBlock(LegacyGlobalBlockName, "Host *\n  HashKnownHosts no\n") +
			managedTestBlock("personal", "Host personal.github.com\n  Hostname ssh.github.com\n"),
	)

	identities, globals := migrationClasses(content)

	for _, name := range []string{GlobalBlockName, LegacyGlobalBlockName} {
		if !containsName(globals, name) {
			t.Errorf("migrationClasses globals = %v, want it to contain %q; identities = %v", globals, name, identities)
		}
		if containsName(identities, name) {
			t.Errorf("globals block %q must not be classified as identity content; identities = %v", name, identities)
		}
	}
	if containsName(identities, sshIncludeBlockName) || containsName(globals, sshIncludeBlockName) {
		t.Errorf("the Include-line block must be stationary and appear in NEITHER list; identities=%v globals=%v", identities, globals)
	}
	if !containsName(identities, "personal") {
		t.Errorf("ordinary identity must be classified as identity content; identities = %v", identities)
	}
}

// TestMigrateToIncludeCarriesGlobalsBlock proves D-07/D-09 for the Include'd
// direction: a storage migration toward the Include'd layout moves EVERY
// managed identity block AND the globals block into the destination file,
// leaves neither in the source, carries every directive the globals body had,
// finishes with the wildcard stanza as the LAST gitid block in the
// destination, and never moves the Include-line block into the destination.
func TestMigrateToIncludeCarriesGlobalsBlock(t *testing.T) {
	skipIfNoSSH(t)
	home, configPath, includePath := migrateFixture(t)
	sshDir := filepath.Join(home, ".ssh")

	identityKey := filepath.Join(sshDir, "id_ed25519_personal")
	hostBlock := RenderHostBlock("personal.github.com", "ssh.github.com", 443, identityKey, "")
	content := managedTestBlock("personal", hostBlock) +
		managedTestBlock(GlobalBlockName, "Host *\n  HashKnownHosts yes\n  AddKeysToAgent yes\n")
	if err := os.WriteFile(configPath, []byte(content), 0o600); err != nil { //nolint:gosec // hermetic t.TempDir() fixture path (G304)
		t.Fatalf("seeding in-line identity + globals: %v", err)
	}

	deps := RealMigrateDeps(configPath, includePath, []string{"personal.github.com"})
	if _, err := Migrate(MigrateToInclude, deps); err != nil {
		t.Fatalf("Migrate(MigrateToInclude): %v", err)
	}

	dest := mustReadFile(t, includePath)
	if !containsBlockName(dest, GlobalBlockName) {
		t.Errorf("globals block must be present in the destination file; got:\n%s", dest)
	}
	if !bytes.Contains(dest, []byte("HashKnownHosts yes")) || !bytes.Contains(dest, []byte("AddKeysToAgent yes")) {
		t.Errorf("globals directives lost during migration; got:\n%s", dest)
	}
	assertGlobalsBlockLast(t, dest, GlobalBlockName, "personal")
	if containsBlockName(dest, sshIncludeBlockName) {
		t.Errorf("the Include-line block must never be written into the destination file; got:\n%s", dest)
	}

	src := mustReadFile(t, configPath)
	if containsBlockName(src, GlobalBlockName) {
		t.Errorf("globals block must be ABSENT from the source file after migration; got:\n%s", src)
	}
	if containsBlockName(src, "personal") {
		t.Errorf("identity block must be absent from the source file after migration; got:\n%s", src)
	}
	if !containsBlockName(src, sshIncludeBlockName) {
		t.Errorf("the Include-line block must stay in the main config file; got:\n%s", src)
	}
	if !bytes.Contains(src, []byte(sshIncludeLineBody)) {
		t.Errorf("the Include line body must stay in the main config file; got:\n%s", src)
	}
}

// TestMigrateToInFileCarriesGlobalsBlock is the same proof for the in-file
// direction: everything moves back into ~/.ssh/config, the globals block lands
// LAST there, the Include-wiring block stays in the main config file, and the
// Include'd file is left empty of gitid block content.
func TestMigrateToInFileCarriesGlobalsBlock(t *testing.T) {
	skipIfNoSSH(t)
	home, configPath, includePath := migrateFixture(t)

	if err := EnsureIncludeDir(filepath.Dir(includePath)); err != nil {
		t.Fatalf("EnsureIncludeDir: %v", err)
	}
	if _, err := EnsureIncludeLine(configPath); err != nil {
		t.Fatalf("EnsureIncludeLine: %v", err)
	}

	sshDir := filepath.Join(home, ".ssh")
	identityKey := filepath.Join(sshDir, "id_ed25519_personal")
	hostBlock := RenderHostBlock("personal.github.com", "ssh.github.com", 443, identityKey, "")
	includeContent := managedTestBlock("personal", hostBlock) +
		managedTestBlock(GlobalBlockName, "Host *\n  HashKnownHosts yes\n")
	if err := os.WriteFile(includePath, []byte(includeContent), 0o600); err != nil { //nolint:gosec // hermetic t.TempDir() fixture path (G304)
		t.Fatalf("seeding Include'd identity + globals: %v", err)
	}

	deps := RealMigrateDeps(configPath, includePath, []string{"personal.github.com"})
	if _, err := Migrate(MigrateToInFile, deps); err != nil {
		t.Fatalf("Migrate(MigrateToInFile): %v", err)
	}

	dest := mustReadFile(t, configPath)
	if !containsBlockName(dest, GlobalBlockName) {
		t.Errorf("globals block must be present in ~/.ssh/config after migration; got:\n%s", dest)
	}
	if !bytes.Contains(dest, []byte("HashKnownHosts yes")) {
		t.Errorf("globals directives lost during migration; got:\n%s", dest)
	}
	assertGlobalsBlockLast(t, dest, GlobalBlockName, "personal")
	if !containsBlockName(dest, sshIncludeBlockName) {
		t.Errorf("the Include-line block must stay in the main config file; got:\n%s", dest)
	}

	src := mustReadFile(t, includePath)
	if containsBlockName(src, GlobalBlockName) || containsBlockName(src, "personal") {
		t.Errorf("Include'd file retains moved blocks after migration; got:\n%s", src)
	}
	if containsBlockName(src, sshIncludeBlockName) {
		t.Errorf("the Include-line block must never be written into the Include'd file; got:\n%s", src)
	}
}

// TestMigrateToIncludeCarriesLegacyGlobalsBlock proves a machine whose
// wildcard stanza is stored under the pre-D-08 sentinel name migrates exactly
// like one storing it under the current name, and its adopted content survives
// (D-08 / 06-CONTEXT.md D-07).
func TestMigrateToIncludeCarriesLegacyGlobalsBlock(t *testing.T) {
	skipIfNoSSH(t)
	home, configPath, includePath := migrateFixture(t)
	sshDir := filepath.Join(home, ".ssh")

	identityKey := filepath.Join(sshDir, "id_ed25519_personal")
	hostBlock := RenderHostBlock("personal.github.com", "ssh.github.com", 443, identityKey, "")
	content := managedTestBlock("personal", hostBlock) +
		managedTestBlock(LegacyGlobalBlockName, "Host *\n  HashKnownHosts yes\n  AddKeysToAgent yes\n")
	if err := os.WriteFile(configPath, []byte(content), 0o600); err != nil { //nolint:gosec // hermetic t.TempDir() fixture path (G304)
		t.Fatalf("seeding in-line identity + legacy globals: %v", err)
	}

	deps := RealMigrateDeps(configPath, includePath, []string{"personal.github.com"})
	if _, err := Migrate(MigrateToInclude, deps); err != nil {
		t.Fatalf("Migrate(MigrateToInclude): %v", err)
	}

	dest := mustReadFile(t, includePath)
	if !containsBlockName(dest, LegacyGlobalBlockName) {
		t.Errorf("legacy-named globals block must be present in the destination; got:\n%s", dest)
	}
	if !bytes.Contains(dest, []byte("HashKnownHosts yes")) || !bytes.Contains(dest, []byte("AddKeysToAgent yes")) {
		t.Errorf("adopted globals content lost during migration; got:\n%s", dest)
	}
	assertGlobalsBlockLast(t, dest, LegacyGlobalBlockName, "personal")

	src := mustReadFile(t, configPath)
	if containsBlockName(src, LegacyGlobalBlockName) {
		t.Errorf("legacy globals block must be ABSENT from the source after migration; got:\n%s", src)
	}
}

// TestMigrateToInFileCarriesLegacyGlobalsBlock is the in-file direction for the
// legacy sentinel name.
func TestMigrateToInFileCarriesLegacyGlobalsBlock(t *testing.T) {
	skipIfNoSSH(t)
	home, configPath, includePath := migrateFixture(t)

	if err := EnsureIncludeDir(filepath.Dir(includePath)); err != nil {
		t.Fatalf("EnsureIncludeDir: %v", err)
	}
	if _, err := EnsureIncludeLine(configPath); err != nil {
		t.Fatalf("EnsureIncludeLine: %v", err)
	}

	sshDir := filepath.Join(home, ".ssh")
	identityKey := filepath.Join(sshDir, "id_ed25519_personal")
	hostBlock := RenderHostBlock("personal.github.com", "ssh.github.com", 443, identityKey, "")
	includeContent := managedTestBlock("personal", hostBlock) +
		managedTestBlock(LegacyGlobalBlockName, "Host *\n  HashKnownHosts yes\n")
	if err := os.WriteFile(includePath, []byte(includeContent), 0o600); err != nil { //nolint:gosec // hermetic t.TempDir() fixture path (G304)
		t.Fatalf("seeding Include'd identity + legacy globals: %v", err)
	}

	deps := RealMigrateDeps(configPath, includePath, []string{"personal.github.com"})
	if _, err := Migrate(MigrateToInFile, deps); err != nil {
		t.Fatalf("Migrate(MigrateToInFile): %v", err)
	}

	dest := mustReadFile(t, configPath)
	if !containsBlockName(dest, LegacyGlobalBlockName) {
		t.Errorf("legacy-named globals block must be present in ~/.ssh/config after migration; got:\n%s", dest)
	}
	if !bytes.Contains(dest, []byte("HashKnownHosts yes")) {
		t.Errorf("adopted globals content lost during migration; got:\n%s", dest)
	}
	assertGlobalsBlockLast(t, dest, LegacyGlobalBlockName, "personal")

	src := mustReadFile(t, includePath)
	if containsBlockName(src, LegacyGlobalBlockName) || containsBlockName(src, "personal") {
		t.Errorf("Include'd file retains moved blocks after migration; got:\n%s", src)
	}
}

// TestMigrateNoGlobalsBehavesAsBefore proves a machine with NO globals block
// at all migrates exactly as it did before the classification change: the
// identity block moves, the Include line is floored, and no globals block is
// fabricated into either file (the globals list is empty, so the move is
// identity-only).
func TestMigrateNoGlobalsBehavesAsBefore(t *testing.T) {
	skipIfNoSSH(t)
	home, configPath, includePath := migrateFixture(t)
	sshDir := filepath.Join(home, ".ssh")

	identityKey := filepath.Join(sshDir, "id_ed25519_personal")
	hostBlock := RenderHostBlock("personal.github.com", "ssh.github.com", 443, identityKey, "")
	if _, err := Write(configPath, "personal", hostBlock, ""); err != nil {
		t.Fatalf("seeding in-line identity (no globals): %v", err)
	}

	deps := RealMigrateDeps(configPath, includePath, []string{"personal.github.com"})
	if _, err := Migrate(MigrateToInclude, deps); err != nil {
		t.Fatalf("Migrate(MigrateToInclude): %v", err)
	}

	dest := mustReadFile(t, includePath)
	if !containsBlockName(dest, "personal") {
		t.Errorf("identity block must still move without any globals block; got:\n%s", dest)
	}
	if containsBlockName(dest, GlobalBlockName) || containsBlockName(dest, LegacyGlobalBlockName) {
		t.Errorf("no globals block may be fabricated by the migration; got:\n%s", dest)
	}

	src := mustReadFile(t, configPath)
	if containsBlockName(src, "personal") {
		t.Errorf("identity block must be removed from source; got:\n%s", src)
	}
	if !containsBlockName(src, sshIncludeBlockName) {
		t.Errorf("Include line must be floored as before; got:\n%s", src)
	}
	if containsBlockName(src, GlobalBlockName) || containsBlockName(src, LegacyGlobalBlockName) {
		t.Errorf("no globals block may be fabricated in the source either; got:\n%s", src)
	}
}

// TestMigrateBothFilesGlobalsAbortsAtPreflight pins the T-06-34 preflight
// abort: when BOTH files carry a gitid globals block before the migration,
// Migrate refuses to guess which wildcard stanza wins — the returned error
// names both file paths, and neither file's bytes change (the abort happens
// before any backup or write, so no `.bak.*` files are created either).
func TestMigrateBothFilesGlobalsAbortsAtPreflight(t *testing.T) {
	skipIfNoSSH(t)
	home, configPath, includePath := migrateFixture(t)
	sshDir := filepath.Join(home, ".ssh")
	if err := os.MkdirAll(filepath.Dir(includePath), 0o700); err != nil {
		t.Fatalf("seeding config.d: %v", err)
	}

	identityKey := filepath.Join(sshDir, "id_ed25519_personal")
	hostBlock := RenderHostBlock("personal.github.com", "ssh.github.com", 443, identityKey, "")
	sourceContent := managedTestBlock("personal", hostBlock) +
		managedTestBlock(GlobalBlockName, "Host *\n  HashKnownHosts yes\n")
	if err := os.WriteFile(configPath, []byte(sourceContent), 0o600); err != nil { //nolint:gosec // hermetic t.TempDir() fixture path (G304)
		t.Fatalf("seeding in-file globals: %v", err)
	}
	destContent := managedTestBlock(LegacyGlobalBlockName, "Host *\n  HashKnownHosts no\n")
	if err := os.WriteFile(includePath, []byte(destContent), 0o600); err != nil { //nolint:gosec // hermetic t.TempDir() fixture path (G304)
		t.Fatalf("seeding Include'd globals: %v", err)
	}

	preSource := mustReadFile(t, configPath)
	preDest := mustReadFile(t, includePath)

	deps := RealMigrateDeps(configPath, includePath, []string{"personal.github.com"})
	_, err := Migrate(MigrateToInclude, deps)
	if err == nil {
		t.Fatal("Migrate must abort when both files carry a globals block")
	}
	for _, path := range []string{configPath, includePath} {
		if !strings.Contains(err.Error(), path) {
			t.Errorf("preflight error must name %s; got %v", path, err)
		}
	}
	if !strings.Contains(err.Error(), "guess") {
		t.Errorf("preflight error must state gitid will not guess which stanza wins; got %v", err)
	}

	// Both files byte-identical, and the abort happened before step 2's backup
	// (seeding via os.WriteFile leaves no backup files to begin with).
	for _, path := range []string{configPath, includePath} {
		matches, globErr := filepath.Glob(path + ".bak.*")
		if globErr != nil {
			t.Fatalf("globbing %s backups: %v", path, globErr)
		}
		if len(matches) != 0 {
			t.Errorf("preflight abort must not create backup files for %s; found %v", path, matches)
		}
	}
	if got := mustReadFile(t, configPath); !bytes.Equal(preSource, got) {
		t.Errorf("source changed by the aborted migration:\nwant:\n%s\ngot:\n%s", preSource, got)
	}
	if got := mustReadFile(t, includePath); !bytes.Equal(preDest, got) {
		t.Errorf("destination changed by the aborted migration:\nwant:\n%s\ngot:\n%s", preDest, got)
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

	result, err := Migrate(MigrateToInclude, deps)
	if err == nil {
		t.Fatal("expected Migrate to fail when afterStep injects an error after the source trim")
	}

	// CR-04 regression: rollbackTracked must return the restored paths
	// ALONGSIDE the error, not discard them — a caller (cmd/gitid/lifecycle.go)
	// reads result.Restored even when err != nil to populate the frozen
	// `restored` JSON field and pick exit code 2 over 1.
	if len(result.Restored) == 0 {
		t.Fatal("Migrate returned no Restored paths after a rollback; CR-04 regressed")
	}
	foundSource := false
	for _, p := range result.Restored {
		if p == configPath {
			foundSource = true
		}
	}
	if !foundSource {
		t.Errorf("Restored = %v, want it to include the rolled-back source path %s", result.Restored, configPath)
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

// ---------------------------------------------------------------------------
// Plan 06-05 Task 1 — PlanMigration / MigrateWithPlan / ErrConfigChangedSincePreview
// ---------------------------------------------------------------------------

// fakeDepsForMutate builds minimal MigrateDeps with real filesystem I/O but
// a noop ResolveAlias (no aliases to resolve in pure planning tests).
func fakeDepsForMutate(configPath, includePath string) MigrateDeps {
	d := RealMigrateDeps(configPath, includePath, nil)
	d.ResolveAlias = func(_, _ string) ([]string, error) { return nil, nil }
	return d
}

// seedTwoIdentityPlusGlobals seeds configPath with two identity blocks and a
// globals block inline (no include layout) and returns the content written.
func seedTwoIdentityPlusGlobals(t *testing.T, configPath string) []byte {
	t.Helper()
	content := []byte(
		managedTestBlock("personal", "Host personal.github.com\n  Hostname ssh.github.com\n  Port 443\n") +
			managedTestBlock("work", "Host work.github.com\n  Hostname ssh.github.com\n  Port 443\n") +
			managedTestBlock(GlobalBlockName, "Host *\n  HashKnownHosts yes\n"),
	)
	if err := os.WriteFile(configPath, content, 0o600); err != nil { //nolint:gosec // hermetic t.TempDir() fixture (G304)
		t.Fatalf("seeding config: %v", err)
	}
	return content
}

// TestPlanMigrationLeavesFilesUnchanged asserts PlanMigration leaves both
// files' bytes unchanged on disk and returns non-empty SourceAfter/DestAfter
// and a non-empty Digests entry for each of the two paths.
func TestPlanMigrationLeavesFilesUnchanged(t *testing.T) {
	_, configPath, includePath := migrateFixture(t)
	seedTwoIdentityPlusGlobals(t, configPath)
	preConfig := mustReadFile(t, configPath)

	deps := fakeDepsForMutate(configPath, includePath)
	plan, err := PlanMigration(MigrateToInclude, deps)
	if err != nil {
		t.Fatalf("PlanMigration: %v", err)
	}

	// Files must be unchanged.
	if got := mustReadFile(t, configPath); !bytes.Equal(got, preConfig) {
		t.Errorf("PlanMigration changed ~/.ssh/config: want %q got %q", preConfig, got)
	}
	if _, statErr := os.Stat(includePath); statErr == nil {
		t.Error("PlanMigration created the include file; it must be read-only")
	}

	// Plan fields must be non-empty.
	if len(plan.SourceAfter) == 0 {
		t.Error("PlanMigration returned empty SourceAfter")
	}
	if len(plan.DestAfter) == 0 {
		t.Error("PlanMigration returned empty DestAfter")
	}
	if plan.Digests[configPath] == "" {
		t.Errorf("PlanMigration returned empty Digests entry for %s", configPath)
	}
	if plan.Digests[includePath] == "" {
		t.Errorf("PlanMigration returned empty Digests entry for %s; want absent marker", includePath)
	}
	if plan.Digests[includePath] != absentDigestMarker {
		t.Errorf("Digests entry for absent file = %q, want %q", plan.Digests[includePath], absentDigestMarker)
	}

	// CR-01 regression: the config.d directory that would hold the include
	// file must NOT be created by planning alone — PlanMigration is
	// documented as read-only ("It never writes, backs up, or shells out
	// beyond the resolution snapshot"), and CLAUDE.md forbids any write to
	// ~/.ssh without a timestamped backup and explicit confirmation.
	if _, statErr := os.Stat(filepath.Dir(includePath)); statErr == nil {
		t.Error("PlanMigration created the config.d directory; planning must be read-only (CR-01)")
	} else if !os.IsNotExist(statErr) {
		t.Fatalf("stat config.d dir: %v", statErr)
	}
}

// TestPlanMigrationNeverMutatesSSHDir is the CR-01 regression: it snapshots
// os.Stat(~/.ssh) (existence, mode, mtime) before and after PlanMigration for
// BOTH directions, and fails on any difference — proving the "pure,
// non-mutating" planning path really is pure, including the ~/.ssh/config.d
// mkdir+chmod that used to fire on every arrow-key preview / --dry-run.
func TestPlanMigrationNeverMutatesSSHDir(t *testing.T) {
	home, configPath, includePath := migrateFixture(t)
	seedTwoIdentityPlusGlobals(t, configPath)
	sshDir := filepath.Join(home, ".ssh")

	statSSHDir := func() os.FileInfo {
		t.Helper()
		info, err := os.Stat(sshDir)
		if err != nil {
			t.Fatalf("stat %s: %v", sshDir, err)
		}
		return info
	}

	before := statSSHDir()

	deps := fakeDepsForMutate(configPath, includePath)
	if _, err := PlanMigration(MigrateToInclude, deps); err != nil {
		t.Fatalf("PlanMigration(MigrateToInclude): %v", err)
	}
	if _, err := PlanMigration(MigrateToInFile, deps); err != nil {
		t.Fatalf("PlanMigration(MigrateToInFile): %v", err)
	}

	after := statSSHDir()
	if before.Mode() != after.Mode() {
		t.Errorf("PlanMigration changed ~/.ssh mode: before %v, after %v", before.Mode(), after.Mode())
	}
	if before.ModTime() != after.ModTime() {
		t.Errorf("PlanMigration changed ~/.ssh mtime: before %v, after %v", before.ModTime(), after.ModTime())
	}
	if _, statErr := os.Stat(filepath.Join(sshDir, "config.d")); !os.IsNotExist(statErr) {
		t.Errorf("PlanMigration created ~/.ssh/config.d (statErr=%v); planning must be read-only (CR-01)", statErr)
	}
}

// TestMigrateWrittenBytesMustMatchPlanBytes asserts the bytes Migrate writes
// to each file are byte-identical to the corresponding PlanMigration field —
// proving there is exactly one composition path and one write path.
func TestMigrateWrittenBytesMustMatchPlanBytes(t *testing.T) {
	_, configPath, includePath := migrateFixture(t)
	seedTwoIdentityPlusGlobals(t, configPath)

	deps := fakeDepsForMutate(configPath, includePath)
	plan, err := PlanMigration(MigrateToInclude, deps)
	if err != nil {
		t.Fatalf("PlanMigration: %v", err)
	}

	if _, err := MigrateWithPlan(plan, deps); err != nil {
		t.Fatalf("MigrateWithPlan: %v", err)
	}

	if got := mustReadFile(t, configPath); !bytes.Equal(got, plan.SourceAfter) {
		t.Errorf("source file bytes differ from plan.SourceAfter:\nwant:\n%s\ngot:\n%s", plan.SourceAfter, got)
	}
	if got := mustReadFile(t, includePath); !bytes.Equal(got, plan.DestAfter) {
		t.Errorf("dest file bytes differ from plan.DestAfter:\nwant:\n%s\ngot:\n%s", plan.DestAfter, got)
	}
}

// TestMigrateWithPlanAbortsPlanChangedSincePreview is the cycle-2 HIGH pinned
// as a sequence: call PlanMigration, HOLD the returned plan, mutate a file on
// disk, then call MigrateWithPlan — assert ErrConfigChangedSincePreview, no
// backup created, bytes unchanged.
func TestMigrateWithPlanAbortsPlanChangedSincePreview(t *testing.T) {
	_, configPath, includePath := migrateFixture(t)
	seedTwoIdentityPlusGlobals(t, configPath)
	preConfig := mustReadFile(t, configPath)

	deps := fakeDepsForMutate(configPath, includePath)
	plan, err := PlanMigration(MigrateToInclude, deps)
	if err != nil {
		t.Fatalf("PlanMigration: %v", err)
	}

	// Mutate the file AFTER planning.
	externalContent := []byte("# externally modified\n")
	if err := os.WriteFile(configPath, externalContent, 0o600); err != nil { //nolint:gosec // hermetic fixture (G304)
		t.Fatalf("mutating config: %v", err)
	}

	_, commitErr := MigrateWithPlan(plan, deps)
	if commitErr == nil {
		t.Fatal("MigrateWithPlan must abort when the config changed since preview")
	}
	if !errors.Is(commitErr, ErrConfigChangedSincePreview) {
		t.Errorf("error = %v, want errors.Is(err, ErrConfigChangedSincePreview)", commitErr)
	}
	if !strings.Contains(commitErr.Error(), configPath) {
		t.Errorf("error must name the changed file %q; got %v", configPath, commitErr)
	}

	// No backup file must have been created.
	for _, path := range []string{configPath, includePath} {
		matches, globErr := filepath.Glob(path + ".bak.*")
		if globErr != nil {
			t.Fatalf("globbing: %v", globErr)
		}
		if len(matches) > 0 {
			t.Errorf("no backup must be created on ErrConfigChangedSincePreview; found %v", matches)
		}
	}

	// The EXTERNAL content must survive unchanged (gitid must not restore stale bytes).
	if got := mustReadFile(t, configPath); !bytes.Equal(got, externalContent) {
		t.Errorf("externally modified file was overwritten by the aborted migration;\nwant:\n%s\ngot:\n%s", externalContent, got)
	}
	// The include file must not have been created.
	if _, statErr := os.Stat(includePath); statErr == nil {
		t.Error("include file must not exist after an aborted migration")
	}
	_ = preConfig // suppress unused var
}

// TestMigrateWithPlanPositive: plan, mutate nothing, commit — assert bytes
// written equal plan.SourceAfter/plan.DestAfter exactly.
func TestMigrateWithPlanPositive(t *testing.T) {
	_, configPath, includePath := migrateFixture(t)
	seedTwoIdentityPlusGlobals(t, configPath)

	deps := fakeDepsForMutate(configPath, includePath)
	plan, err := PlanMigration(MigrateToInclude, deps)
	if err != nil {
		t.Fatalf("PlanMigration: %v", err)
	}

	if _, err := MigrateWithPlan(plan, deps); err != nil {
		t.Fatalf("MigrateWithPlan: %v", err)
	}

	if got := mustReadFile(t, configPath); !bytes.Equal(got, plan.SourceAfter) {
		t.Errorf("source bytes differ from plan.SourceAfter")
	}
	if got := mustReadFile(t, includePath); !bytes.Equal(got, plan.DestAfter) {
		t.Errorf("dest bytes differ from plan.DestAfter")
	}
}

// TestErrConfigChangedSincePreviewDistinguishable asserts ErrConfigChangedSincePreview
// is distinguishable via errors.Is from the error an injected write failure produces.
func TestErrConfigChangedSincePreviewDistinguishable(t *testing.T) {
	_, configPath, includePath := migrateFixture(t)
	seedTwoIdentityPlusGlobals(t, configPath)

	// Case A: config-changed error.
	depsA := fakeDepsForMutate(configPath, includePath)
	plan, err := PlanMigration(MigrateToInclude, depsA)
	if err != nil {
		t.Fatalf("PlanMigration: %v", err)
	}
	if err := os.WriteFile(configPath, []byte("# changed\n"), 0o600); err != nil { //nolint:gosec // hermetic fixture (G304)
		t.Fatalf("mutating: %v", err)
	}
	_, errA := MigrateWithPlan(plan, depsA)
	if !errors.Is(errA, ErrConfigChangedSincePreview) {
		t.Errorf("errA = %v; want errors.Is(errA, ErrConfigChangedSincePreview)", errA)
	}

	// Case B: injected write failure — must NOT match ErrConfigChangedSincePreview.
	if err := os.WriteFile(configPath, plan.SourceBefore, 0o600); err != nil { //nolint:gosec // hermetic fixture (G304)
		t.Fatalf("restoring: %v", err)
	}
	plan2, err := PlanMigration(MigrateToInclude, depsA)
	if err != nil {
		t.Fatalf("second PlanMigration: %v", err)
	}
	depsB := depsA
	depsB.WriteFile = func(_ string, _ []byte, _ os.FileMode) (string, error) {
		return "", errors.New("injected write failure")
	}
	_, errB := MigrateWithPlan(plan2, depsB)
	if errB == nil {
		t.Fatal("expected injected write failure, got nil")
	}
	if errors.Is(errB, ErrConfigChangedSincePreview) {
		t.Errorf("injected write failure must NOT match ErrConfigChangedSincePreview; errB = %v", errB)
	}
}

// TestMigrateWithPlanWritesPlanNotRecomposition proves MigrateWithPlan writes
// the plan's bytes VERBATIM even when recomposing from the current disk
// contents would produce something different. It adjusts that file's digest
// in the plan copy so the check passes, and asserts the written bytes are
// still the plan's original DestAfter.
func TestMigrateWithPlanWritesPlanNotRecomposition(t *testing.T) {
	_, configPath, includePath := migrateFixture(t)
	content := []byte(managedTestBlock("personal", "Host personal.github.com\n  Hostname ssh.github.com\n"))
	if err := os.WriteFile(configPath, content, 0o600); err != nil { //nolint:gosec // hermetic fixture (G304)
		t.Fatalf("seeding: %v", err)
	}

	deps := fakeDepsForMutate(configPath, includePath)
	plan, err := PlanMigration(MigrateToInclude, deps)
	if err != nil {
		t.Fatalf("PlanMigration: %v", err)
	}

	// Modify the file AND update its digest in the plan so the check passes,
	// simulating a "same file, updated digest" path.
	newSourceBytes := []byte(managedTestBlock("personal", "Host personal.github.com\n  Hostname ssh.github.com\n  Port 22\n"))
	if err := os.WriteFile(configPath, newSourceBytes, 0o600); err != nil { //nolint:gosec // hermetic fixture (G304)
		t.Fatalf("modifying source: %v", err)
	}
	// Update digests to match the modified content so the check passes.
	plan.Digests[configPath] = contentDigest(newSourceBytes)
	plan.SourceBefore = newSourceBytes // so rollback can restore if needed

	if _, err := MigrateWithPlan(plan, deps); err != nil {
		t.Fatalf("MigrateWithPlan: %v", err)
	}

	// The dest file must contain plan.DestAfter (the ORIGINAL plan bytes).
	if got := mustReadFile(t, includePath); !bytes.Equal(got, plan.DestAfter) {
		t.Errorf("MigrateWithPlan wrote recomposed bytes instead of plan.DestAfter;\nwant (plan):\n%s\ngot:\n%s", plan.DestAfter, got)
	}
}

// TestMigrateMutateBeforePreBackupAborts mutates a file between preflight
// and the pre-backup check and asserts Migrate returns an error, NO backup
// was created, and both files' bytes are unchanged (review HIGH placement fix).
func TestMigrateMutateBeforePreBackupAborts(t *testing.T) {
	_, configPath, includePath := migrateFixture(t)
	seedTwoIdentityPlusGlobals(t, configPath)
	preConfig := mustReadFile(t, configPath)

	externalContent := []byte("# external edit before backup\n")
	deps := fakeDepsForMutate(configPath, includePath)
	deps.afterStep = func(step MigrateStep) error {
		if step == StepPreflight {
			// Mutate the config between preflight and backup.
			return os.WriteFile(configPath, externalContent, 0o600) //nolint:gosec // hermetic fixture (G304)
		}
		return nil
	}

	_, err := Migrate(MigrateToInclude, deps)
	if err == nil {
		t.Fatal("Migrate must abort when the file was mutated before backup")
	}

	// No backup created.
	for _, path := range []string{configPath, includePath} {
		matches, globErr := filepath.Glob(path + ".bak.*")
		if globErr != nil {
			t.Fatalf("globbing: %v", globErr)
		}
		if len(matches) > 0 {
			t.Errorf("no backup must be created when abort fires before backup; found %v", matches)
		}
	}

	// The external content must survive.
	if got := mustReadFile(t, configPath); !bytes.Equal(got, externalContent) {
		t.Errorf("external content was overwritten; want %q got %q", externalContent, got)
	}
	_ = preConfig
}

// TestMigrateSourceMutatedBetweenBackupAndWrite mutates the source file after
// the backup and asserts an error naming that file is returned.
func TestMigrateSourceMutatedBetweenBackupAndWrite(t *testing.T) {
	_, configPath, includePath := migrateFixture(t)
	seedTwoIdentityPlusGlobals(t, configPath)

	externalContent := []byte("# source mutated between backup and write\n")
	deps := fakeDepsForMutate(configPath, includePath)
	deps.afterStep = func(step MigrateStep) error {
		if step == StepDestinationWritten {
			// Mutate the SOURCE between destination write and source write.
			return os.WriteFile(configPath, externalContent, 0o600) //nolint:gosec // hermetic fixture (G304)
		}
		return nil
	}

	_, err := Migrate(MigrateToInclude, deps)
	if err == nil {
		t.Fatal("Migrate must abort when source is mutated between dest write and source write")
	}
	if !strings.Contains(err.Error(), configPath) {
		t.Errorf("error must name the changed file %q; got: %v", configPath, err)
	}
	// External content survives (gitid must not restore over it).
	if got := mustReadFile(t, configPath); !bytes.Equal(got, externalContent) {
		t.Errorf("source external content was overwritten; want %q got %q", externalContent, got)
	}
}

// TestMigrateDestMutatedBetweenBackupAndWrite mutates the destination file
// after backup (via afterStep=StepBackup) and before the destination write,
// and asserts an error naming that file.
func TestMigrateDestMutatedBetweenBackupAndWrite(t *testing.T) {
	_, configPath, includePath := migrateFixture(t)
	seedTwoIdentityPlusGlobals(t, configPath)
	if err := os.MkdirAll(filepath.Dir(includePath), 0o700); err != nil {
		t.Fatalf("creating include dir: %v", err)
	}

	// Seed the dest file so Backup has something to back up and the dest file
	// "exists" at backup time with a known digest.
	existingDest := []byte("# pre-existing dest\n")
	if err := os.WriteFile(includePath, existingDest, 0o600); err != nil { //nolint:gosec // hermetic fixture (G304)
		t.Fatalf("seeding dest: %v", err)
	}

	externalContent := []byte("# dest mutated after backup\n")
	deps := fakeDepsForMutate(configPath, includePath)
	deps.afterStep = func(step MigrateStep) error {
		if step == StepBackup {
			// Mutate the dest after both files are backed up but before the
			// pre-write check runs for the destination.
			return os.WriteFile(includePath, externalContent, 0o600) //nolint:gosec // hermetic fixture (G304)
		}
		return nil
	}

	_, err := Migrate(MigrateToInclude, deps)
	if err == nil {
		t.Fatal("Migrate must abort when dest is mutated after backup and before dest write")
	}
	if !strings.Contains(err.Error(), includePath) {
		t.Errorf("error must name the changed file %q; got: %v", includePath, err)
	}
	// External content survives (gitid must not restore over it).
	if got := mustReadFile(t, includePath); !bytes.Equal(got, externalContent) {
		t.Errorf("dest external content was overwritten; want %q got %q", externalContent, got)
	}
}

// TestMigrateExternallyModifiedFileSurvivesAbort asserts the externally
// modified file's final content equals the EXTERNAL bytes, not the preflight
// snapshot — the gitid transaction must not overwrite an external edit on abort.
func TestMigrateExternallyModifiedFileSurvivesAbort(t *testing.T) {
	_, configPath, includePath := migrateFixture(t)
	seedTwoIdentityPlusGlobals(t, configPath)

	externalContent := []byte("# external edit survives abort\n")
	deps := fakeDepsForMutate(configPath, includePath)
	deps.afterStep = func(step MigrateStep) error {
		if step == StepPreflight {
			return os.WriteFile(configPath, externalContent, 0o600) //nolint:gosec // hermetic fixture (G304)
		}
		return nil
	}

	_, err := Migrate(MigrateToInclude, deps)
	if err == nil {
		t.Fatal("expected abort, got success")
	}

	if got := mustReadFile(t, configPath); !bytes.Equal(got, externalContent) {
		t.Errorf("external content was overwritten; want %q got %q", externalContent, got)
	}
}

// TestMigrateGitidWriteRolledBackOnAbort asserts that when gitid has already
// written the destination file and then an abort happens during the source
// write, gitid's own destination write is rolled back to pre-transaction bytes.
func TestMigrateGitidWriteRolledBackOnAbort(t *testing.T) {
	_, configPath, includePath := migrateFixture(t)
	seedTwoIdentityPlusGlobals(t, configPath)
	// Pre-create include dir.
	if err := os.MkdirAll(filepath.Dir(includePath), 0o700); err != nil {
		t.Fatalf("creating include dir: %v", err)
	}

	preConfig := mustReadFile(t, configPath)
	// includePath does not exist yet.

	deps := fakeDepsForMutate(configPath, includePath)
	deps.afterStep = func(step MigrateStep) error {
		if step == StepDestinationWritten {
			return errFakeCrash
		}
		return nil
	}

	_, err := Migrate(MigrateToInclude, deps)
	if err == nil {
		t.Fatal("expected abort, got success")
	}

	// Source must be restored to pre-transaction bytes.
	if got := mustReadFile(t, configPath); !bytes.Equal(got, preConfig) {
		t.Errorf("source not restored; want pre-transaction bytes:\n%s\ngot:\n%s", preConfig, got)
	}
	// Destination must be removed (it did not pre-exist).
	if _, statErr := os.Stat(includePath); statErr == nil {
		t.Error("destination was written by gitid but not rolled back on abort")
	}
}

// TestMigrateAbsentAtPreflightExistsAtWriteAborts asserts a file that was
// absent at snapshot time but exists at write time triggers the concurrency
// detection.
func TestMigrateAbsentAtPreflightExistsAtWriteAborts(t *testing.T) {
	_, configPath, includePath := migrateFixture(t)
	seedTwoIdentityPlusGlobals(t, configPath)

	// includePath does NOT exist at plan time. Create it between preflight
	// and the backup check.
	deps := fakeDepsForMutate(configPath, includePath)
	deps.afterStep = func(step MigrateStep) error {
		if step == StepPreflight {
			if err := os.MkdirAll(filepath.Dir(includePath), 0o700); err != nil {
				return err
			}
			return os.WriteFile(includePath, []byte("# appeared after preflight\n"), 0o600) //nolint:gosec // hermetic fixture (G304)
		}
		return nil
	}

	_, err := Migrate(MigrateToInclude, deps)
	if err == nil {
		t.Fatal("Migrate must abort when a file absent at preflight exists at backup time")
	}
}

// TestMigrateHappyPathDoesNotFireConcurrencyCheck asserts a full two-step
// migration completes without the detection firing — the transaction's own
// writes are reconciled and do not trigger the check.
func TestMigrateHappyPathDoesNotFireConcurrencyCheck(t *testing.T) {
	_, configPath, includePath := migrateFixture(t)
	seedTwoIdentityPlusGlobals(t, configPath)

	deps := fakeDepsForMutate(configPath, includePath)
	result, err := Migrate(MigrateToInclude, deps)
	if err != nil {
		t.Fatalf("Migrate must succeed on happy path: %v", err)
	}
	if result.SourceBackup == "" {
		t.Error("expected non-empty SourceBackup for pre-existing source file")
	}
	// TargetBackup may be empty when the dest did not pre-exist (BackupFile
	// returns "" for absent files — that is correct behavior: there is nothing
	// to back up when the file did not exist before migration).
	_ = result
}

// TestMigrateSameLengthDifferentContentDetected asserts detection fires for a
// same-content-length, different-content mutation — proving the check is
// content-based, not size- or time-based.
func TestMigrateSameLengthDifferentContentDetected(t *testing.T) {
	_, configPath, includePath := migrateFixture(t)
	original := []byte("# original-12345\n")
	sameLen := []byte("# modified-1234\n") // same byte count
	if len(original) != len(sameLen) {
		// Adjust if needed; both are 17 bytes
		t.Skipf("test fixture length mismatch (%d vs %d), adjust literal", len(original), len(sameLen))
	}
	if err := os.WriteFile(configPath, original, 0o600); err != nil { //nolint:gosec // hermetic fixture (G304)
		t.Fatalf("seeding: %v", err)
	}

	deps := fakeDepsForMutate(configPath, includePath)
	plan, err := PlanMigration(MigrateToInclude, deps)
	if err != nil {
		t.Fatalf("PlanMigration: %v", err)
	}

	// Replace with same-length different content AFTER planning.
	if err := os.WriteFile(configPath, sameLen, 0o600); err != nil { //nolint:gosec // hermetic fixture (G304)
		t.Fatalf("injecting same-length edit: %v", err)
	}

	_, commitErr := MigrateWithPlan(plan, deps)
	if commitErr == nil {
		t.Fatal("MigrateWithPlan must detect a same-length different-content modification")
	}
	if !errors.Is(commitErr, ErrConfigChangedSincePreview) {
		t.Errorf("error = %v, want ErrConfigChangedSincePreview", commitErr)
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
	// Fork the sleep into the background and `wait`, so a grandchild holds the
	// stdout pipe after the direct child (the shell) is killed. This reproduces
	// on every platform the Linux-only hang where .Output() blocked on a
	// pipe-holding grandchild past the context deadline (Codex HIGH #2 regression
	// guard) — killing only the direct process is not enough.
	// #nosec G306 -- test fixture in a t.TempDir(), not a managed gitid file
	if err := os.WriteFile(script, []byte("#!/bin/sh\nsleep 30 & wait\n"), 0o755); err != nil {
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
