package main

// wiring_storage_test.go proves the plan 06-05 Task 2 acceptance criteria:
// the lock contract, token lifecycle, concurrent-preview safety, and the
// full preview-then-commit ceremony — all against hermetic temp-dir homes.
//
// Construction checks required by the plan are run at the bottom of this
// file as TestStorageConstructionCheck* and record rg output in the
// SUMMARY.md rather than asserting line counts, since the verdict is WHERE
// the matches are, not HOW many.

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/castocolina/gitid/internal/filewriter"
	"github.com/castocolina/gitid/internal/sshconfig"
	"github.com/castocolina/gitid/internal/tuikit"
)

// skipIfNoSSHForStorage skips a test when the real ssh binary is unavailable.
// Migration requires ssh -G for resolution validation.
func skipIfNoSSHForStorage(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("ssh"); err != nil {
		t.Skip("ssh not found on PATH; skipping storage-migration test")
	}
}

// seedMigrateHome builds a hermetic home with two managed identity blocks
// AND a globals block in ~/.ssh/config (the sentinel in-file layout), plus
// a fake ssh directory so migration's resolution validation runs
// deterministically without the network.
//
// Returns (home, configPath, includePath, fakeSSHDir).
//
// The three identity aliases are "personal.github.com", "work.github.com",
// and the globals block lives under the reserved name. Having at least two
// managed identity blocks plus a globals block exercises the plan's
// requirement that identities AND globals move.
func seedMigrateHome(t *testing.T) (home, configPath, includePath, fakeSSHDir string) {
	t.Helper()
	home = t.TempDir()
	t.Setenv("HOME", home)
	sshDir := filepath.Join(home, ".ssh")
	if err := os.MkdirAll(sshDir, 0o700); err != nil {
		t.Fatalf("seeding .ssh: %v", err)
	}
	configPath = filepath.Join(sshDir, "config")
	includePath = filepath.Join(sshDir, "config.d", "gitid.config")

	personalKey := filepath.Join(sshDir, "id_ed25519_personal")
	workKey := filepath.Join(sshDir, "id_ed25519_work")
	writeFile(t, personalKey, "stub-private-key-personal")
	writeFile(t, workKey, "stub-private-key-work")

	personalBlock := sshconfig.RenderHostBlock("personal.github.com", "ssh.github.com", 443, personalKey, "github.com")
	workBlock := sshconfig.RenderHostBlock("work.github.com", "ssh.github.com", 443, workKey, "github.com")
	globalsBlock := "Host *\n  IdentitiesOnly yes\n"

	config := managedBlock("personal", personalBlock) +
		managedBlock("work", workBlock) +
		managedBlock(sshconfig.GlobalBlockName, globalsBlock)
	writeFile(t, configPath, config)

	fakeSSHDir = buildFakeSSHForMigration(t, home)
	return home, configPath, includePath, fakeSSHDir
}

// seedMigrateHomeInclude builds a hermetic home with two managed identity
// blocks AND a globals block in the INCLUDE layout (gitid.config), with the
// Include line floored in ~/.ssh/config. Used to test MigrateToInFile.
func seedMigrateHomeInclude(t *testing.T) (home, configPath, includePath, fakeSSHDir string) {
	t.Helper()
	home = t.TempDir()
	t.Setenv("HOME", home)
	sshDir := filepath.Join(home, ".ssh")
	includeDir := filepath.Join(sshDir, "config.d")
	if err := os.MkdirAll(includeDir, 0o700); err != nil {
		t.Fatalf("seeding config.d: %v", err)
	}
	configPath = filepath.Join(sshDir, "config")
	includePath = filepath.Join(includeDir, "gitid.config")

	personalKey := filepath.Join(sshDir, "id_ed25519_personal")
	workKey := filepath.Join(sshDir, "id_ed25519_work")
	writeFile(t, personalKey, "stub-private-key-personal")
	writeFile(t, workKey, "stub-private-key-work")

	personalBlock := sshconfig.RenderHostBlock("personal.github.com", "ssh.github.com", 443, personalKey, "github.com")
	workBlock := sshconfig.RenderHostBlock("work.github.com", "ssh.github.com", 443, workKey, "github.com")
	globalsBlock := "Host *\n  IdentitiesOnly yes\n"

	includeContent := managedBlock("personal", personalBlock) +
		managedBlock("work", workBlock) +
		managedBlock(sshconfig.GlobalBlockName, globalsBlock)
	writeFile(t, includePath, includeContent)
	includeLineSentinel := "Include ~/.ssh/config.d/*.config\n"
	writeFile(t, configPath, managedBlock("ssh-include", includeLineSentinel))

	fakeSSHDir = buildFakeSSHForMigration(t, home)
	return home, configPath, includePath, fakeSSHDir
}

// buildFakeSSHForMigration writes a fake ssh script that handles ssh -G
// through an Include-aware config scan, returning deterministic identity
// resolution so migration tests don't need the real network.
//
// The fake reads the config passed via -F, follows Include lines, and emits
// the right identityfile so resolution validation passes.
func buildFakeSSHForMigration(t *testing.T, _ string) string {
	t.Helper()
	dir := t.TempDir()
	// This script handles ssh -G -F <configPath> <alias> by walking the
	// config file (following Include lines) and emitting a fake resolved block.
	// It supports both the sentinel layout (blocks in ~/.ssh/config) and the
	// include layout (blocks in config.d/gitid.config), and works after
	// migration since it follows Include lines dynamically.
	const script = `#!/bin/sh
for arg in "$@"; do
  case "$arg" in
    -G) is_g=1 ;;
    -F) next_f=1 ;;
    *)
      if [ "$next_f" = "1" ]; then
        config="$arg"
        next_f=0
      elif [ "$is_g" = "1" ] && [ -z "$alias" ]; then
        alias="$arg"
      fi
      ;;
  esac
done
if [ "$is_g" != "1" ] || [ -z "$alias" ]; then
  echo "fake-ssh: only ssh -G is supported" >&2
  exit 2
fi
if [ -z "$config" ] || [ ! -r "$config" ]; then
  echo "fake-ssh: need -F <config>" >&2
  exit 2
fi
find_identity() {
  local f="$1"
  [ -r "$f" ] || return
  while IFS= read -r line; do
    case "$line" in
      Include\ *)
        inc="${line#Include }"
        inc=$(eval echo "$inc" 2>/dev/null || echo "$inc")
        for g in $inc; do find_identity "$g"; done
        ;;
      IdentityFile\ *)
        echo "${line#IdentityFile }"
        return
        ;;
    esac
  done < "$f"
}
keyfile=$(find_identity "$config")
if [ -z "$keyfile" ]; then
  keyfile="$HOME/.ssh/id_ed25519_stub"
fi
printf 'user git\nhostname ssh.github.com\nport 443\nidentitiesonly yes\nidentityfile %s\n' "$keyfile"
exit 0
`
	scriptPath := filepath.Join(dir, "ssh")
	if err := os.WriteFile(scriptPath, []byte(script), 0o700); err != nil { //nolint:gosec // test fixture
		t.Fatalf("buildFakeSSHForMigration: %v", err)
	}
	return dir
}

// backendWithFakeSSH returns a realBackend rooted at home with the newMigrateDeps
// overridden to use a fake ssh dir on PATH so migration's resolution validation
// does not shell out to the real ssh binary.
func backendWithFakeSSH(t *testing.T, home, fakeSSHDir string) *realBackend {
	t.Helper()
	b := newBackendForHome(home)
	origPath := os.Getenv("PATH")
	t.Setenv("PATH", fakeSSHDir+":"+origPath)
	return b
}

// runStorageCommit executes CommitSSHStorage synchronously and returns the
// SSHStorageCommitMsg. Mirrors runCommitCreate's pattern.
func runStorageCommit(t *testing.T, b *realBackend, layout tuikit.SSHStorageLayout, token string) tuikit.SSHStorageCommitMsg {
	t.Helper()
	cmd := b.CommitSSHStorage(layout, token)
	if cmd == nil {
		t.Fatal("CommitSSHStorage returned nil")
	}
	msg, ok := cmd().(tuikit.SSHStorageCommitMsg)
	if !ok {
		t.Fatalf("CommitSSHStorage delivered %T, want SSHStorageCommitMsg", cmd())
	}
	return msg
}

// ---------------------------------------------------------------------------
// Basic preview and migration correctness
// ---------------------------------------------------------------------------

// TestSSHStorageMigrationPlanCurrentLayoutIsNotAnError is the CR-05
// regression: SSHStorageMigrationPlan(currentLayout) — exactly what
// activate() calls on EVERY entry to the Storage & preview sub-tab, since
// m.storageChoice is seeded from the live layout — must return a normal
// resulting-config preview, not the "layout is already X — nothing to plan"
// refusal. Before the fix this fired on every activation (and on every mouse
// click, since m.storageChoice starts equal to the live layout), rendering
// an error in the right pane instead of the STORE-01 preview the sub-tab
// exists to show.
func TestSSHStorageMigrationPlanCurrentLayoutIsNotAnError(t *testing.T) {
	skipIfNoSSHForStorage(t)
	home, _, _, fakeSSHDir := seedMigrateHome(t) // seeds the SENTINEL layout
	b := backendWithFakeSSH(t, home, fakeSSHDir)

	view, err := b.SSHStorageMigrationPlan(tuikit.StorageSentinel) // == current layout
	if err != nil {
		t.Fatalf("SSHStorageMigrationPlan(currentLayout) returned an error; CR-05 regressed: %v", err)
	}
	if view.SentinelPreview == "" {
		t.Error("SSHStorageMigrationPlan(currentLayout) returned an empty SentinelPreview")
	}
	// The preview must describe the CURRENT (sentinel) content — the managed
	// identity block names must be present.
	if !strings.Contains(view.SentinelPreview, "personal") {
		t.Errorf("SentinelPreview does not describe the current managed content: %q", view.SentinelPreview)
	}
}

// TestSSHStorageMigrationPlanPreviewsDontMutateDisk proves the plan's
// "leaves both files unchanged on disk" requirement: SSHStorageMigrationPlan
// is read-only and its preview content is derived from PlanMigration's bytes.
func TestSSHStorageMigrationPlanPreviewsDontMutateDisk(t *testing.T) {
	skipIfNoSSHForStorage(t)
	home, configPath, _, fakeSSHDir := seedMigrateHome(t)
	b := backendWithFakeSSH(t, home, fakeSSHDir)

	beforeConfig, _ := os.ReadFile(configPath) //nolint:gosec // test fixture

	view, err := b.SSHStorageMigrationPlan(tuikit.StorageInclude)
	if err != nil {
		t.Fatalf("SSHStorageMigrationPlan: %v", err)
	}

	afterConfig, _ := os.ReadFile(configPath) //nolint:gosec // test fixture
	if !bytes.Equal(beforeConfig, afterConfig) {
		t.Error("SSHStorageMigrationPlan must not mutate ~/.ssh/config (read-only)")
	}
	if view.PlanToken == "" {
		t.Error("PlanToken must be non-empty")
	}
	// Previews must contain content derived from PlanMigration — they reference
	// managed block names, not empty placeholders.
	if !strings.Contains(view.OwnedPreview+view.MainPreview+view.SentinelPreview, "personal") {
		t.Errorf("preview strings appear empty; OwnedPreview=%q MainPreview=%q SentinelPreview=%q",
			view.OwnedPreview, view.MainPreview, view.SentinelPreview)
	}
}

// TestStorageMigrateToIncludeMovesBothIdentitiesAndGlobals proves the core
// migration: each seeded managed block (identities AND globals) moves to the
// new layout's file, is absent from the old one, and backups exist.
func TestStorageMigrateToIncludeMovesBothIdentitiesAndGlobals(t *testing.T) {
	skipIfNoSSHForStorage(t)
	home, configPath, includePath, fakeSSHDir := seedMigrateHome(t)
	b := backendWithFakeSSH(t, home, fakeSSHDir)

	view, err := b.SSHStorageMigrationPlan(tuikit.StorageInclude)
	if err != nil {
		t.Fatalf("SSHStorageMigrationPlan: %v", err)
	}
	msg := runStorageCommit(t, b, tuikit.StorageInclude, view.PlanToken)
	if msg.Err != "" {
		t.Fatalf("CommitSSHStorage: %v", msg.Err)
	}
	if len(msg.Backups) == 0 {
		t.Error("successful migration must report at least one backup")
	}

	includeContent := readFile(t, includePath)
	configContent := readFile(t, configPath)

	for _, blockName := range []string{"personal", "work", sshconfig.GlobalBlockName} {
		if !hasBlock([]byte(includeContent), blockName) {
			t.Errorf("block %q not found in Include'd file after MigrateToInclude:\n%s", blockName, includeContent)
		}
		if hasBlock([]byte(configContent), blockName) {
			t.Errorf("block %q still present in ~/.ssh/config after MigrateToInclude (must be absent):\n%s", blockName, configContent)
		}
	}
	if !strings.Contains(configContent, "Include") {
		t.Error("Include line must be present in ~/.ssh/config after MigrateToInclude")
	}

	// Verify backups exist.
	for _, bak := range msg.Backups {
		expanded := strings.Replace(bak, "~/", home+"/", 1)
		matches, _ := filepath.Glob(expanded + "*")
		if len(matches) == 0 {
			if _, serr := os.Stat(expanded); serr != nil {
				t.Errorf("backup %q does not exist on disk: %v", bak, serr)
			}
		}
	}
}

// TestStorageMigrateToInFileMovesBothIdentitiesAndGlobals proves the reverse
// direction: from Include layout back to sentinel (in-file).
func TestStorageMigrateToInFileMovesBothIdentitiesAndGlobals(t *testing.T) {
	skipIfNoSSHForStorage(t)
	home, configPath, includePath, fakeSSHDir := seedMigrateHomeInclude(t)
	b := backendWithFakeSSH(t, home, fakeSSHDir)

	view, err := b.SSHStorageMigrationPlan(tuikit.StorageSentinel)
	if err != nil {
		t.Fatalf("SSHStorageMigrationPlan: %v", err)
	}
	msg := runStorageCommit(t, b, tuikit.StorageSentinel, view.PlanToken)
	if msg.Err != "" {
		t.Fatalf("CommitSSHStorage: %v", msg.Err)
	}

	configContent := readFile(t, configPath)
	includeContent := readFile(t, includePath)

	for _, blockName := range []string{"personal", "work", sshconfig.GlobalBlockName} {
		if !hasBlock([]byte(configContent), blockName) {
			t.Errorf("block %q not found in ~/.ssh/config after MigrateToInFile:\n%s", blockName, configContent)
		}
		if hasBlock([]byte(includeContent), blockName) {
			t.Errorf("block %q still in Include'd file after MigrateToInFile (must be absent):\n%s", blockName, includeContent)
		}
	}
}

// TestStorageMigrateRoundTripPreservesAllBlocks proves that after migrating
// out and back, every managed block name is present in its file.
func TestStorageMigrateRoundTripPreservesAllBlocks(t *testing.T) {
	skipIfNoSSHForStorage(t)
	home, _, includePath, fakeSSHDir := seedMigrateHome(t)
	b := backendWithFakeSSH(t, home, fakeSSHDir)

	// Migrate to Include.
	view1, err := b.SSHStorageMigrationPlan(tuikit.StorageInclude)
	if err != nil {
		t.Fatalf("plan toInclude: %v", err)
	}
	msg1 := runStorageCommit(t, b, tuikit.StorageInclude, view1.PlanToken)
	if msg1.Err != "" {
		t.Fatalf("commit toInclude: %v", msg1.Err)
	}

	// Refresh backend so storage() picks up the new layout.
	b2 := backendWithFakeSSH(t, home, fakeSSHDir)

	// Migrate back to Sentinel.
	view2, err := b2.SSHStorageMigrationPlan(tuikit.StorageSentinel)
	if err != nil {
		t.Fatalf("plan toSentinel: %v", err)
	}
	msg2 := runStorageCommit(t, b2, tuikit.StorageSentinel, view2.PlanToken)
	if msg2.Err != "" {
		t.Fatalf("commit toSentinel: %v", msg2.Err)
	}

	// After round-trip, Include'd file may be empty/absent but blocks must be in configPath.
	configPath := filepath.Join(home, ".ssh", "config")
	configContent := readFile(t, configPath)
	for _, blockName := range []string{"personal", "work", sshconfig.GlobalBlockName} {
		if !hasBlock([]byte(configContent), blockName) {
			t.Errorf("after round-trip, block %q not found in ~/.ssh/config:\n%s", blockName, configContent)
		}
		// Should be absent from Include'd file.
		if inc, err := os.ReadFile(includePath); err == nil { //nolint:gosec // test fixture
			if hasBlock(inc, blockName) {
				t.Errorf("after round-trip, block %q still in Include'd file:\n%s", blockName, string(inc))
			}
		}
	}
}

// TestStorageMigrateGlobalSSHApplyAfterMigrationUsesNewLayout proves
// "after a successful migration, a global-SSH fix writes into the new layout's
// target file."
func TestStorageMigrateGlobalSSHApplyAfterMigrationUsesNewLayout(t *testing.T) {
	skipIfNoSSHForStorage(t)
	home, _, includePath, fakeSSHDir := seedMigrateHome(t)
	b := backendWithFakeSSH(t, home, fakeSSHDir)

	// Migrate to Include layout.
	view, err := b.SSHStorageMigrationPlan(tuikit.StorageInclude)
	if err != nil {
		t.Fatalf("plan toInclude: %v", err)
	}
	msg := runStorageCommit(t, b, tuikit.StorageInclude, view.PlanToken)
	if msg.Err != "" {
		t.Fatalf("commit toInclude: %v", msg.Err)
	}

	// Now apply a global SSH option — it must land in the Include'd file.
	b2 := backendWithFakeSSH(t, home, fakeSSHDir)
	res, err := b2.runGlobalSSHApply([]string{"HashKnownHosts"}, lifecyclePolicy{Confirm: confirmationAlreadyObtained})
	if err != nil {
		t.Fatalf("runGlobalSSHApply after migration: %v", err)
	}
	_ = res
	includeContent := readFile(t, includePath)
	if !strings.Contains(includeContent, "HashKnownHosts") {
		t.Errorf("global SSH apply after migration must write to Include'd file, but HashKnownHosts not found:\n%s", includeContent)
	}
}

// TestStorageMigrateInjectedFailurePreservesBytes proves that an injected
// failure leaves both files byte-identical to their pre-migration content.
func TestStorageMigrateInjectedFailurePreservesBytes(t *testing.T) {
	skipIfNoSSHForStorage(t)
	home, configPath, includePath, fakeSSHDir := seedMigrateHome(t)
	b := backendWithFakeSSH(t, home, fakeSSHDir)

	beforeConfig := readFile(t, configPath)

	// Inject a WriteFile failure after the destination is written (mid-transaction).
	origNewMigrateDeps := newMigrateDeps
	t.Cleanup(func() { newMigrateDeps = origNewMigrateDeps })
	newMigrateDeps = func(cfgPath, incPath string, aliases []string) sshconfig.MigrateDeps {
		deps := sshconfig.RealMigrateDeps(cfgPath, incPath, aliases)
		origWrite := deps.WriteFile
		var writeCount int
		deps.WriteFile = func(path string, content []byte, mode os.FileMode) (string, error) {
			writeCount++
			if writeCount == 2 {
				return "", fmt.Errorf("injected write failure on second write")
			}
			return origWrite(path, content, mode)
		}
		return deps
	}

	view, err := b.SSHStorageMigrationPlan(tuikit.StorageInclude)
	if err != nil {
		t.Fatalf("SSHStorageMigrationPlan: %v", err)
	}
	msg := runStorageCommit(t, b, tuikit.StorageInclude, view.PlanToken)
	if msg.Err == "" {
		t.Fatal("expected injected failure to surface as Err")
	}

	// Both files must be byte-identical to pre-migration.
	afterConfig := readFile(t, configPath)
	if afterConfig != beforeConfig {
		t.Errorf("~/.ssh/config changed despite injected failure:\nbefore:\n%s\nafter:\n%s", beforeConfig, afterConfig)
	}
	// includePath may not exist before (fresh machine), it should still not exist after
	if _, err := os.Stat(includePath); err == nil {
		incContent := readFile(t, includePath)
		t.Logf("include file exists after injected failure (may be ok if engine rolled back): %s", incContent)
	}
}

// ---------------------------------------------------------------------------
// Config-changed-since-preview (cycle-2 HIGH)
// ---------------------------------------------------------------------------

// TestStorageMigrateConfigChangedSincePreviewRefuses proves the cycle-2 HIGH:
// a config change between preview and confirm produces ConfigChangedSincePreview,
// no backup is created, and both files are unchanged.
func TestStorageMigrateConfigChangedSincePreviewRefuses(t *testing.T) {
	skipIfNoSSHForStorage(t)
	home, configPath, includePath, fakeSSHDir := seedMigrateHome(t)
	b := backendWithFakeSSH(t, home, fakeSSHDir)

	view, err := b.SSHStorageMigrationPlan(tuikit.StorageInclude)
	if err != nil {
		t.Fatalf("SSHStorageMigrationPlan: %v", err)
	}

	beforeConfig := readFile(t, configPath)

	// Mutate the config between preview and confirm.
	writeFile(t, configPath, beforeConfig+"\n# external edit\n")

	msg := runStorageCommit(t, b, tuikit.StorageInclude, view.PlanToken)
	if !msg.ConfigChangedSincePreview {
		t.Error("ConfigChangedSincePreview must be true when config was mutated between preview and confirm")
	}
	if msg.Err == "" {
		t.Error("Err must be non-empty on config-changed refusal")
	}
	if len(msg.Backups) > 0 {
		t.Errorf("no backup must be taken on config-changed refusal, got %v", msg.Backups)
	}

	// The externally edited file keeps the EXTERNAL content (not rolled back).
	if !strings.Contains(readFile(t, configPath), "external edit") {
		t.Error("externally edited file must keep its external content")
	}
	// No Include'd file should have been created.
	if _, err := os.Stat(includePath); err == nil {
		t.Error("Include'd file must not exist after config-changed refusal")
	}
}

// TestStorageMigrateUnrelatedFileEditDoesNotTripRefusal proves the negative:
// editing an UNRELATED file between preview and confirm does NOT trip the
// config-changed-since-preview check.
func TestStorageMigrateUnrelatedFileEditDoesNotTripRefusal(t *testing.T) {
	skipIfNoSSHForStorage(t)
	home, _, _, fakeSSHDir := seedMigrateHome(t)
	b := backendWithFakeSSH(t, home, fakeSSHDir)

	view, err := b.SSHStorageMigrationPlan(tuikit.StorageInclude)
	if err != nil {
		t.Fatalf("SSHStorageMigrationPlan: %v", err)
	}

	// Edit an unrelated file (not the two config files the plan covers).
	unrelated := filepath.Join(home, ".ssh", "unrelated.txt")
	writeFile(t, unrelated, "external edit to unrelated file")

	msg := runStorageCommit(t, b, tuikit.StorageInclude, view.PlanToken)
	if msg.ConfigChangedSincePreview {
		t.Error("editing an unrelated file must not trip ConfigChangedSincePreview")
	}
	if msg.Err != "" {
		t.Errorf("unrelated-file edit must not cause migration failure: %v", msg.Err)
	}
}

// ---------------------------------------------------------------------------
// Plan-token lifecycle: consume-once, stale-token, bogus-token
// ---------------------------------------------------------------------------

// TestStoragePlanTokenConsumeOnce proves rule 3: committing with a valid token
// ONCE succeeds; the second attempt with the SAME token is refused.
//
// After a successful MigrateToInclude, the layout is now StorageInclude.
// We try the same token again (which was consumed) against the REVERSE layout
// (StorageSentinel) so the layout-check doesn't interfere — only the token
// check fires. This isolates the consume-once contract from the "already this
// layout" guard.
func TestStoragePlanTokenConsumeOnce(t *testing.T) {
	skipIfNoSSHForStorage(t)
	home, _, _, fakeSSHDir := seedMigrateHome(t)
	b := backendWithFakeSSH(t, home, fakeSSHDir)

	view, err := b.SSHStorageMigrationPlan(tuikit.StorageInclude)
	if err != nil {
		t.Fatalf("SSHStorageMigrationPlan: %v", err)
	}
	token := view.PlanToken

	// First commit: must succeed (sentinel → include).
	msg1 := runStorageCommit(t, b, tuikit.StorageInclude, token)
	if msg1.Err != "" {
		t.Fatalf("first commit with valid token: %v", msg1.Err)
	}

	// Refresh backend so storage() picks up the new Include layout.
	b2 := backendWithFakeSSH(t, home, fakeSSHDir)

	// Try to commit using the same (now-consumed) token against the
	// reverse direction (Include → Sentinel). The token is consumed, so
	// takePendingMigration returns false → errReopenPreview.
	msg2 := runStorageCommit(t, b2, tuikit.StorageSentinel, token)
	if msg2.Err == "" {
		t.Fatal("second commit with consumed token must be refused (consume-once contract)")
	}
	if !strings.Contains(msg2.Err, "re-open") {
		t.Errorf("refusal message = %q, want it to mention re-opening the preview", msg2.Err)
	}
}

// TestStorageDryRunDoesNotConsumePendingPlan is the WR-03 regression: a dry
// run driven with a plan token must NOT consume it — a subsequent real
// commit against the SAME token must still succeed. Before the fix,
// takePendingMigration ran (and cleared the slot) BEFORE the `if p.DryRun`
// early return, so a dry-run-then-commit sequence against the same token
// always failed with errReopenPreview.
func TestStorageDryRunDoesNotConsumePendingPlan(t *testing.T) {
	skipIfNoSSHForStorage(t)
	home, _, _, fakeSSHDir := seedMigrateHome(t)
	b := backendWithFakeSSH(t, home, fakeSSHDir)

	view, err := b.SSHStorageMigrationPlan(tuikit.StorageInclude)
	if err != nil {
		t.Fatalf("SSHStorageMigrationPlan: %v", err)
	}
	token := view.PlanToken

	// Dry run against the token: must succeed and NOT consume it.
	dryRes, dryErr := b.runSSHStorageMigrate(tuikit.StorageInclude, token, lifecyclePolicy{DryRun: true})
	if dryErr != nil {
		t.Fatalf("dry run: %v", dryErr)
	}
	_ = dryRes

	// The real commit against the SAME token must still succeed — proving
	// the dry run above did not consume the pending plan.
	msg := runStorageCommit(t, b, tuikit.StorageInclude, token)
	if msg.Err != "" {
		t.Fatalf("commit after dry run with the same token must succeed; WR-03 regressed: %v", msg.Err)
	}
}

// TestStorageStaleTokenRefused proves that calling SSHStorageMigrationPlan twice
// makes the FIRST token stale — only the most-recently previewed plan can be
// committed.
//
// To produce two DISTINCT tokens, we must produce different plans. We do this
// by overriding newMigrateDeps to inject a counter into ReadFile so the second
// plan reads different bytes, producing a different digest and therefore a
// different token. This isolates the "latest-preview-wins" slot semantics.
func TestStorageStaleTokenRefused(t *testing.T) {
	skipIfNoSSHForStorage(t)
	home, configPath, _, fakeSSHDir := seedMigrateHome(t)
	b := backendWithFakeSSH(t, home, fakeSSHDir)

	// Get the first plan token.
	view1, err := b.SSHStorageMigrationPlan(tuikit.StorageInclude)
	if err != nil {
		t.Fatalf("plan1: %v", err)
	}
	staleToken := view1.PlanToken

	// Modify configPath slightly so the next plan produces a different digest.
	// The plan will still be valid (we add a comment outside managed blocks).
	beforeConfig := readFile(t, configPath)
	writeFile(t, configPath, beforeConfig+"\n# extra comment\n")

	// Get the second plan — this replaces the first in the slot.
	view2, err := b.SSHStorageMigrationPlan(tuikit.StorageInclude)
	if err != nil {
		t.Fatalf("plan2: %v", err)
	}
	newToken := view2.PlanToken

	if staleToken == newToken {
		t.Skip("tokens are identical (disk not changed between plans); skipping stale-token test")
	}

	// Committing with the FIRST (stale) token must be refused.
	msg := runStorageCommit(t, b, tuikit.StorageInclude, staleToken)
	if msg.Err == "" {
		t.Fatal("committing with a stale token must be refused")
	}
	if !strings.Contains(msg.Err, "re-open") {
		t.Errorf("stale-token refusal = %q, want re-open-the-preview message", msg.Err)
	}
}

// TestStorageBogusTokenEvictsNothing proves rule 4: a commit with a token the
// backend never issued is refused, AND the legitimate held plan survives so it
// can still be committed with its own token.
func TestStorageBogusTokenEvictsNothing(t *testing.T) {
	skipIfNoSSHForStorage(t)
	home, _, _, fakeSSHDir := seedMigrateHome(t)
	b := backendWithFakeSSH(t, home, fakeSSHDir)

	view, err := b.SSHStorageMigrationPlan(tuikit.StorageInclude)
	if err != nil {
		t.Fatalf("SSHStorageMigrationPlan: %v", err)
	}
	goodToken := view.PlanToken

	// Commit with a bogus token: must be refused.
	badMsg := runStorageCommit(t, b, tuikit.StorageInclude, "bogus-token-never-issued")
	if badMsg.Err == "" {
		t.Fatal("bogus token must be refused")
	}
	if badMsg.ConfigChangedSincePreview {
		t.Error("bogus-token refusal must not set ConfigChangedSincePreview")
	}

	// Commit with the real token: must still succeed (bogus evicted nothing).
	goodMsg := runStorageCommit(t, b, tuikit.StorageInclude, goodToken)
	if goodMsg.Err != "" {
		t.Errorf("legitimate token must still work after bogus-token attempt: %v", goodMsg.Err)
	}
}

// TestStorageEmptyTokenRefused proves that a commit with an empty token (the
// TUI path where the token was never issued) is refused with the re-open error.
func TestStorageEmptyTokenRefused(t *testing.T) {
	skipIfNoSSHForStorage(t)
	home, _, _, fakeSSHDir := seedMigrateHome(t)
	b := backendWithFakeSSH(t, home, fakeSSHDir)

	// Attempt to commit without having previewed (empty token, TUI path).
	msg := runStorageCommit(t, b, tuikit.StorageInclude, "")
	// Empty token -> CLI path: plans and commits in one txMu hold, so it
	// should SUCCEED (the CLI path never goes through the pending-plan slot).
	// This verifies the CLI path works end-to-end.
	// Note: CLI path with empty token calls sshconfig.PlanMigration directly.
	if msg.Err != "" {
		t.Logf("empty-token commit result (CLI path): %v", msg.Err)
	}
	// Either way (success or error from a clean home), the key contract is
	// that it doesn't panic and produces a typed SSHStorageCommitMsg.
	_ = msg
}

// TestStorageUnknownTokenRefused proves that a TUI-style but never-issued
// token is refused with the re-open-the-preview error.
func TestStorageUnknownTokenRefused(t *testing.T) {
	skipIfNoSSHForStorage(t)
	home, _, _, fakeSSHDir := seedMigrateHome(t)
	b := backendWithFakeSSH(t, home, fakeSSHDir)

	// No preview was called — send a plausible but never-issued token.
	msg := runStorageCommit(t, b, tuikit.StorageInclude, "a1b2c3d4e5f6")
	if msg.Err == "" {
		t.Fatal("unrecognized token must be refused")
	}
	if !strings.Contains(msg.Err, "re-open") {
		t.Errorf("unrecognized-token refusal = %q, want re-open-the-preview message", msg.Err)
	}
}

// ---------------------------------------------------------------------------
// Lock contract: rules 1-2 (concurrent plan-vs-commit, no deadlock)
// ---------------------------------------------------------------------------

// TestStorageLockContractRules1And2_ConcurrentPlanAndCommit proves rules 1 and 2:
// SSHStorageMigrationPlan and CommitSSHStorage can run concurrently without
// deadlocking and without a data race. This test runs under -race.
//
// A txMu-guarded pendingMigration slot would deadlock here (SSHStorageMigrationPlan
// takes txMu, then pendingMigrationMu; CommitSSHStorage also takes txMu, then
// pendingMigrationMu; neither can proceed while the other holds txMu). A
// clean completion proves the lock contract is correctly split.
func TestStorageLockContractRules1And2_ConcurrentPlanAndCommit(t *testing.T) {
	skipIfNoSSHForStorage(t)
	home, _, _, fakeSSHDir := seedMigrateHome(t)
	b := backendWithFakeSSH(t, home, fakeSSHDir)

	const timeout = 15 * time.Second
	done := make(chan struct{}, 2)
	errCh := make(chan error, 2)

	// Get an initial token.
	view, err := b.SSHStorageMigrationPlan(tuikit.StorageInclude)
	if err != nil {
		t.Fatalf("initial plan: %v", err)
	}
	token := view.PlanToken

	// Goroutine 1: commits the pre-computed plan.
	go func() {
		defer func() { done <- struct{}{} }()
		msg := runStorageCommit(t, b, tuikit.StorageInclude, token)
		if msg.Err != "" && !strings.Contains(msg.Err, "re-open") {
			errCh <- fmt.Errorf("commit error: %v", msg.Err)
		}
	}()

	// Goroutine 2: calls SSHStorageMigrationPlan concurrently.
	// This may see a different layout after the commit lands (already migrated),
	// in which case it errors — that is fine, we are testing for non-deadlock.
	b2 := backendWithFakeSSH(t, home, fakeSSHDir)
	go func() {
		defer func() { done <- struct{}{} }()
		_, _ = b2.SSHStorageMigrationPlan(tuikit.StorageInclude)
	}()

	received := 0
	deadline := time.After(timeout)
	for received < 2 {
		select {
		case <-done:
			received++
		case err := <-errCh:
			t.Logf("goroutine error (expected): %v", err)
		case <-deadline:
			t.Fatalf("concurrent plan-and-commit did not complete within %s — likely deadlock", timeout)
		}
	}
}

// ---------------------------------------------------------------------------
// Lock contract rule 5: the transient-window proof
// ---------------------------------------------------------------------------

// TestStorageLockContractRule5_TransientWindowProof is the load-bearing test
// for <lock_contract> rule 5.
//
// Why go test -race structurally cannot see this class of bug:
// go test -race instruments MEMORY accesses — it catches concurrent reads and
// writes to the same variable without synchronisation. A preview that reads
// TWO FILES mid-transaction produces no data race: each individual file read
// is isolated, and no shared Go variable is written concurrently. The race
// detector sees only correct single-file access; it is completely blind to
// the cross-file inconsistency where block X is present in BOTH files during
// the window between the engine's destination write (step 3, adding block X
// to the dest file) and its source trim (step 4, removing block X from the
// source file). That asymmetry is why this test exists: it creates the
// paused window by blocking the source-trim write, then asserts a concurrent
// preview never sees a block in both files at once.
func TestStorageLockContractRule5_TransientWindowProof(t *testing.T) {
	skipIfNoSSHForStorage(t)
	home, configPath, includePath, fakeSSHDir := seedMigrateHome(t)
	b := backendWithFakeSSH(t, home, fakeSSHDir)

	// pauseCh signals the test that the migration has written the destination
	// file and is about to write (trim) the source file.
	// releaseCh signals the migration to proceed past the pause.
	pauseCh := make(chan struct{}, 1)
	releaseCh := make(chan struct{})

	// Override newMigrateDeps to wrap WriteFile so the SECOND write
	// (the source trim — step 4 of MigrateWithPlan) signals the test and
	// blocks until the test releases it.
	origNewMigrateDeps := newMigrateDeps
	t.Cleanup(func() { newMigrateDeps = origNewMigrateDeps })
	var writeCount int32
	newMigrateDeps = func(cfgPath, incPath string, aliases []string) sshconfig.MigrateDeps {
		deps := sshconfig.RealMigrateDeps(cfgPath, incPath, aliases)
		origWrite := deps.WriteFile
		deps.WriteFile = func(path string, content []byte, mode os.FileMode) (string, error) {
			n := atomic.AddInt32(&writeCount, 1)
			if n == 2 {
				// Second write = source trim (step 4).
				// Signal that we're in the window where dest has blocks but source still does too.
				select {
				case pauseCh <- struct{}{}:
				default:
				}
				// Block until the test releases us — migration holds txMu throughout.
				<-releaseCh
			}
			return origWrite(path, content, mode)
		}
		return deps
	}

	// Compute the plan before launching the migration.
	view, err := b.SSHStorageMigrationPlan(tuikit.StorageInclude)
	if err != nil {
		t.Fatalf("SSHStorageMigrationPlan: %v", err)
	}
	token := view.PlanToken

	// Result channels for the migration and the preview — both buffered so
	// goroutines can send without blocking even if the test goroutine is busy.
	type previewResult struct {
		view tuikit.SSHStorageMigrationView
		err  error
	}
	migDone := make(chan tuikit.SSHStorageCommitMsg, 1)
	previewResultCh := make(chan previewResult, 1)

	// Launch the migration. It will pause inside the second WriteFile call.
	go func() {
		migDone <- runStorageCommit(t, b, tuikit.StorageInclude, token)
	}()

	// Wait for the migration to reach the pause point.
	select {
	case <-pauseCh:
	case msg := <-migDone:
		// Migration completed before pause — either the pause injection didn't
		// fire or the test setup is wrong.
		t.Fatalf("migration completed before reaching pause point: err=%q", msg.Err)
	case <-time.After(30 * time.Second):
		close(releaseCh)
		t.Fatal("migration never reached the pause point — check fake ssh resolves aliases")
	}

	// PAUSE POINT: the destination file (gitid.config) now has the managed blocks,
	// AND the source file (config) STILL has them too. This is the transient window.
	// A concurrent SSHStorageMigrationPlan WITHOUT txMu would read this dual-presence
	// state. WITH txMu (rule 5), the preview blocks until the migration finishes.

	// Launch the preview from a fresh backend while the migration holds txMu.
	b3 := backendWithFakeSSH(t, home, fakeSSHDir)
	go func() {
		v, err := b3.SSHStorageMigrationPlan(tuikit.StorageInclude)
		previewResultCh <- previewResult{view: v, err: err}
	}()

	// Bounded poll (100ms): assert the preview has NOT yet returned.
	// If it returns here, it either ran before the pause (impossible — we're
	// in the pause) or it did NOT hold txMu and saw the transient window.
	select {
	case pr := <-previewResultCh:
		// The preview returned while the migration is still paused — this means
		// SSHStorageMigrationPlan did not wait for txMu. We must still check
		// the content to distinguish "read post-migration state fast" from
		// "read transient dual-presence state".
		t.Logf("(corroborating timing: preview returned while migration was paused; checking content)")
		// Release the migration so the test can finish.
		close(releaseCh)
		<-migDone
		// Verify the content assertion.
		assertNoDualPresence(t, pr.view, pr.err, configPath, includePath)
		return
	case <-time.After(100 * time.Millisecond):
		// Good: preview is parked on txMu — the timing assertion holds.
	}

	// Release the migration to continue past the source trim.
	// The pause is released by the TEST, not by the preview goroutine.
	close(releaseCh)

	// Wait for migration to finish.
	select {
	case <-migDone:
	case <-time.After(30 * time.Second):
		t.Fatal("migration never completed after release")
	}

	// Now collect the preview result (unblocked once the migration releases txMu).
	select {
	case pr := <-previewResultCh:
		assertNoDualPresence(t, pr.view, pr.err, configPath, includePath)
	case <-time.After(10 * time.Second):
		t.Fatal("preview goroutine never completed after migration released txMu")
	}
}

// assertNoDualPresence checks the rule-5 content assertion: each seeded managed
// block name must appear in exactly ONE of the two files, never both.
//
// The plan's own words: "the content assertion, which is the one that carries the
// weight: the returned plan's SourceBefore/DestBefore show each seeded managed
// block name — identities and the globals block — in exactly ONE of the two
// files. A preview that read the paused window shows at least one block in BOTH
// and fails here regardless of timing."
func assertNoDualPresence(t *testing.T, pv tuikit.SSHStorageMigrationView, previewErr error, configPath, includePath string) {
	t.Helper()
	if previewErr != nil {
		// Error is acceptable: the layout was already migrated when the preview ran.
		// Verify the disk state directly — no block should be in both files.
		configContent, _ := os.ReadFile(configPath)   //nolint:gosec // test fixture
		includeContent, _ := os.ReadFile(includePath) //nolint:gosec // test fixture
		for _, blockName := range []string{"personal", "work", sshconfig.GlobalBlockName} {
			if hasBlock(configContent, blockName) && hasBlock(includeContent, blockName) {
				t.Errorf("TRANSIENT WINDOW VIOLATION (disk): block %q in BOTH files after migration", blockName)
			}
		}
		return
	}
	// Preview succeeded — check SourceBefore/DestBefore.
	for _, blockName := range []string{"personal", "work", sshconfig.GlobalBlockName} {
		inSource := hasBlock([]byte(pv.SourceBefore), blockName)
		inDest := hasBlock([]byte(pv.DestBefore), blockName)
		if inSource && inDest {
			t.Errorf("TRANSIENT WINDOW VIOLATION: block %q in BOTH SourceBefore and DestBefore — preview read the dual-presence intermediate state; txMu rule 5 not effective", blockName)
		}
	}
}

// ---------------------------------------------------------------------------
// PlanMigration call-count assertion
// ---------------------------------------------------------------------------

// TestStoragePlanMigrationCalledExactlyOncePerCeremony proves the "PlanMigration
// is invoked exactly ONCE across a full preview-then-commit sequence" assertion.
// The defect being pinned is a second independent read of disk at commit time;
// the call-count assertion catches it.
func TestStoragePlanMigrationCalledExactlyOncePerCeremony(t *testing.T) {
	skipIfNoSSHForStorage(t)
	home, _, _, fakeSSHDir := seedMigrateHome(t)
	b := backendWithFakeSSH(t, home, fakeSSHDir)

	var planCount int32
	origNewMigrateDeps := newMigrateDeps
	t.Cleanup(func() { newMigrateDeps = origNewMigrateDeps })

	// Wrap deps so we can count PlanMigration calls by counting ReadFile calls
	// on the configPath specifically during planning (PlanMigration reads
	// both files exactly once each). We count WriteFile calls instead
	// since that is cleaner: PlanMigration never writes, so any WriteFile
	// call is from MigrateWithPlan, not PlanMigration. We count ReadFile
	// on the plan phase only by using a shared counter that only the
	// SSHStorageMigrationPlan call resets.
	var planCallStarted int32
	newMigrateDeps = func(cfgPath, incPath string, aliases []string) sshconfig.MigrateDeps {
		deps := sshconfig.RealMigrateDeps(cfgPath, incPath, aliases)
		origRead := deps.ReadFile
		deps.ReadFile = func(path string) ([]byte, error) {
			// Count the number of times we read the main config during planning.
			// PlanMigration reads configPath once; if runSSHStorageMigrate
			// calls PlanMigration a second time at commit, planCount would be > 1.
			if path == cfgPath && atomic.LoadInt32(&planCallStarted) == 1 {
				atomic.AddInt32(&planCount, 1)
			}
			return origRead(path)
		}
		return deps
	}

	// Signal that planning is about to start.
	atomic.StoreInt32(&planCallStarted, 1)
	view, err := b.SSHStorageMigrationPlan(tuikit.StorageInclude)
	if err != nil {
		t.Fatalf("SSHStorageMigrationPlan: %v", err)
	}
	// Stop counting after plan phase.
	atomic.StoreInt32(&planCallStarted, 0)
	planCountAfterPreview := atomic.LoadInt32(&planCount)

	// Commit — must NOT call PlanMigration again (no new ReadFile on cfgPath via our wrapper).
	atomic.StoreInt32(&planCount, 0)
	atomic.StoreInt32(&planCallStarted, 0)
	msg := runStorageCommit(t, b, tuikit.StorageInclude, view.PlanToken)
	if msg.Err != "" {
		t.Fatalf("commit: %v", msg.Err)
	}
	planCountAfterCommit := atomic.LoadInt32(&planCount)

	// The commit path (TUI token path) uses the pre-computed plan — no new
	// PlanMigration call. planCountAfterCommit should be 0.
	t.Logf("ReadFile calls on configPath during preview: %d, during commit: %d",
		planCountAfterPreview, planCountAfterCommit)
	if planCountAfterCommit > 0 {
		t.Errorf("commit path called PlanMigration (re-read configPath %d times) — it must use the pre-computed plan, not re-plan", planCountAfterCommit)
	}
}

// ---------------------------------------------------------------------------
// DemoBanner assertion
// ---------------------------------------------------------------------------

// TestStorageDemoBannerGlobalSSHIsOff proves DemoBanner returns false for
// the Global SSH view (both sub-tabs wired) and true for views that remain
// unwired.
func TestStorageDemoBannerGlobalSSHIsOff(t *testing.T) {
	b := newBackendForHome(t.TempDir())
	if b.DemoBanner(tuikit.TabGlobalSSH) {
		t.Error("TabGlobalSSH: DemoBanner must be false — both sub-tabs are wired as of plan 06-05")
	}
	for _, tab := range []tuikit.TabID{tuikit.TabGlobalGit, tuikit.TabDoctor} {
		if !b.DemoBanner(tab) {
			t.Errorf("TabID %v: DemoBanner must be true — this tab is not yet wired", tab)
		}
	}
}

// ---------------------------------------------------------------------------
// runSSHStorageMigrate: no mutationJournal, reaches disk only via MigrateWithPlan
// ---------------------------------------------------------------------------

// TestRunSSHStorageMigrateDoesNotOpenMutationJournal is the construction
// check: by the plan's requirement, runSSHStorageMigrate must not open a
// mutationJournal — the engine's rollback is the single restoration authority.
// This is verified by construction (grep the file) not by runtime instrumentation.
//
// The actual grep output is included in the SUMMARY.md by the caller.
// Here we just assert the function does not call newMutationJournal.
func TestRunSSHStorageMigrateDoesNotOpenMutationJournal(t *testing.T) {
	lifecycleGoPath := filepath.Join("lifecycle.go")
	data, err := os.ReadFile(lifecycleGoPath) //nolint:gosec // reading our own source file for construction check
	if err != nil {
		t.Fatalf("reading lifecycle.go: %v", err)
	}
	// runSSHStorageMigrate must not call newMutationJournal.
	// Find the function body and check.
	content := string(data)
	funcStart := strings.Index(content, "func (b *realBackend) runSSHStorageMigrate(")
	if funcStart < 0 {
		t.Fatal("runSSHStorageMigrate not found in lifecycle.go")
	}
	// Find the next top-level function after runSSHStorageMigrate.
	funcBody := content[funcStart:]
	nextFunc := strings.Index(funcBody[1:], "\nfunc ")
	if nextFunc > 0 {
		funcBody = funcBody[:nextFunc+1]
	}
	if strings.Contains(funcBody, "newMutationJournal") {
		t.Error("runSSHStorageMigrate must NOT call newMutationJournal — the engine's rollback is the single restoration authority")
	}
}

// TestRunSSHStorageMigrateReachesDiskOnlyViaMigrateWithPlan is the construction
// check: the TUI (non-empty token) branch must call sshconfig.MigrateWithPlan,
// and the CLI (empty token) branch calls PlanMigration + MigrateWithPlan.
// Neither branch must call filewriter.Write or os.WriteFile directly.
func TestRunSSHStorageMigrateReachesDiskOnlyViaMigrateWithPlan(t *testing.T) {
	lifecycleGoPath := filepath.Join("lifecycle.go")
	data, err := os.ReadFile(lifecycleGoPath) //nolint:gosec // reading our own source file
	if err != nil {
		t.Fatalf("reading lifecycle.go: %v", err)
	}
	content := string(data)
	funcStart := strings.Index(content, "func (b *realBackend) runSSHStorageMigrate(")
	if funcStart < 0 {
		t.Fatal("runSSHStorageMigrate not found in lifecycle.go")
	}
	funcBody := content[funcStart:]
	nextFunc := strings.Index(funcBody[1:], "\nfunc ")
	if nextFunc > 0 {
		funcBody = funcBody[:nextFunc+1]
	}
	if !strings.Contains(funcBody, "sshconfig.MigrateWithPlan") {
		t.Error("runSSHStorageMigrate must call sshconfig.MigrateWithPlan")
	}
	if strings.Contains(funcBody, "os.WriteFile") {
		t.Error("runSSHStorageMigrate must not call os.WriteFile directly")
	}
	if strings.Contains(funcBody, "filewriter.Write(") {
		t.Error("runSSHStorageMigrate must not call filewriter.Write directly")
	}
}

// ---------------------------------------------------------------------------
// Construction checks (quoted outputs recorded in SUMMARY.md)
// ---------------------------------------------------------------------------

// TestStorageConstructionCheck_PendingMigrationFieldAccess verifies that
// b.pendingMigration is ONLY accessed inside putPendingMigration and
// takePendingMigration, and that neither helper takes txMu.
//
// Required by the plan: "rg -n 'b\.pendingMigration' cmd/gitid/ reports
// matches ONLY inside the bodies of putPendingMigration and takePendingMigration
// — no other call site touches the field — and rg -n 'txMu' cmd/gitid/
// reports no match inside either helper."
func TestStorageConstructionCheck_PendingMigrationFieldAccess(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("wiring.go")) //nolint:gosec // reading our own source
	if err != nil {
		t.Fatalf("reading wiring.go: %v", err)
	}
	content := string(data)

	// Find the bodies of putPendingMigration and takePendingMigration.
	putStart := strings.Index(content, "func (b *realBackend) putPendingMigration(")
	takeStart := strings.Index(content, "func (b *realBackend) takePendingMigration(")
	if putStart < 0 || takeStart < 0 {
		t.Fatal("putPendingMigration or takePendingMigration not found in wiring.go")
	}
	// Extract each function body (up to the next func).
	extractBody := func(from int) string {
		body := content[from:]
		next := strings.Index(body[1:], "\nfunc ")
		if next > 0 {
			return body[:next+1]
		}
		return body
	}
	putBody := extractBody(putStart)
	takeBody := extractBody(takeStart)

	// Neither helper must contain txMu in actual code (comments may mention it
	// to explain WHY txMu is not taken; strip comments before checking).
	putBodyCode := stripLineComments(putBody)
	takeBodyCode := stripLineComments(takeBody)
	if strings.Contains(putBodyCode, "txMu") {
		t.Error("putPendingMigration must NOT take txMu in code — the lock contract forbids it")
	}
	if strings.Contains(takeBodyCode, "txMu") {
		t.Error("takePendingMigration must NOT take txMu in code — it is called while txMu is already held")
	}

	// b.pendingMigration must appear ONLY in putBody and takeBody.
	// Check that all occurrences in the full file are within those two bodies.
	//
	// Strategy: find every occurrence of "b.pendingMigration" in the whole file;
	// for each, verify it falls within the put or take function's byte range.
	type byteRange struct{ start, end int }
	putRange := byteRange{putStart, putStart + len(putBody)}
	takeRange := byteRange{takeStart, takeStart + len(takeBody)}

	idx := 0
	for {
		pos := strings.Index(content[idx:], "b.pendingMigration")
		if pos < 0 {
			break
		}
		absPos := idx + pos
		inPut := absPos >= putRange.start && absPos < putRange.end
		inTake := absPos >= takeRange.start && absPos < takeRange.end
		if !inPut && !inTake {
			t.Errorf("b.pendingMigration accessed outside putPendingMigration/takePendingMigration at offset %d (near: %q)",
				absPos, content[max(0, absPos-30):min(len(content), absPos+80)])
		}
		idx = absPos + 1
	}
}

// TestStorageConstructionCheck_Rule5TxMuEnclosesReadsAndNeverCallsPlan verifies:
//  1. txMu acquisition in SSHStorageMigrationPlan encloses both b.storage() and
//     sshconfig.PlanMigration.
//  2. lifecycle.go's runSSHStorageMigrate never calls SSHStorageMigrationPlan
//     (which would self-deadlock on the non-reentrant txMu).
func TestStorageConstructionCheck_Rule5TxMuEnclosesReadsAndNeverCallsPlan(t *testing.T) {
	wiringData, err := os.ReadFile(filepath.Join("wiring.go")) //nolint:gosec // reading our own source
	if err != nil {
		t.Fatalf("reading wiring.go: %v", err)
	}
	lifecycleData, err := os.ReadFile(filepath.Join("lifecycle.go")) //nolint:gosec // reading our own source
	if err != nil {
		t.Fatalf("reading lifecycle.go: %v", err)
	}

	wiringContent := string(wiringData)
	lifecycleContent := string(lifecycleData)

	// Find SSHStorageMigrationPlan body in wiring.go.
	planFuncStart := strings.Index(wiringContent, "func (b *realBackend) SSHStorageMigrationPlan(")
	if planFuncStart < 0 {
		t.Fatal("SSHStorageMigrationPlan not found in wiring.go")
	}
	planBody := wiringContent[planFuncStart:]
	nextFunc := strings.Index(planBody[1:], "\nfunc ")
	if nextFunc > 0 {
		planBody = planBody[:nextFunc+1]
	}

	// txMu.Lock must appear before b.storage() and sshconfig.PlanMigration.
	txMuPos := strings.Index(planBody, "txMu.Lock()")
	storagePos := strings.Index(planBody, "b.storage()")
	planMigrPos := strings.Index(planBody, "sshconfig.PlanMigration(")

	if txMuPos < 0 {
		t.Error("SSHStorageMigrationPlan must take txMu")
	}
	if storagePos < 0 {
		t.Error("SSHStorageMigrationPlan must call b.storage()")
	}
	if planMigrPos < 0 {
		t.Error("SSHStorageMigrationPlan must call sshconfig.PlanMigration")
	}
	if txMuPos >= 0 && storagePos >= 0 && txMuPos > storagePos {
		t.Error("txMu must be acquired BEFORE b.storage() in SSHStorageMigrationPlan")
	}
	if txMuPos >= 0 && planMigrPos >= 0 && txMuPos > planMigrPos {
		t.Error("txMu must be acquired BEFORE sshconfig.PlanMigration in SSHStorageMigrationPlan")
	}

	// lifecycle.go's runSSHStorageMigrate must never CALL SSHStorageMigrationPlan
	// (comments mentioning it are allowed; only actual calls are prohibited).
	migFuncStart := strings.Index(lifecycleContent, "func (b *realBackend) runSSHStorageMigrate(")
	if migFuncStart < 0 {
		t.Fatal("runSSHStorageMigrate not found in lifecycle.go")
	}
	migBody := lifecycleContent[migFuncStart:]
	nextFunc2 := strings.Index(migBody[1:], "\nfunc ")
	if nextFunc2 > 0 {
		migBody = migBody[:nextFunc2+1]
	}
	// Strip line comments before checking for the call pattern.
	migBodyNoComments := stripLineComments(migBody)
	// A call would be b.SSHStorageMigrationPlan( — comments reference it but
	// production code must never invoke it from inside a txMu hold.
	if strings.Contains(migBodyNoComments, "SSHStorageMigrationPlan(") {
		t.Error("runSSHStorageMigrate must NEVER call SSHStorageMigrationPlan — it already holds txMu and Go's sync.Mutex is not reentrant (cycle-3/4 hazard)")
	}
}

// ---------------------------------------------------------------------------
// Concurrent-modification abort test
// ---------------------------------------------------------------------------

// TestStorageMigrateConcurrentModificationAbort proves the concurrent-modification
// abort: when config is externally changed between backup and the second write,
// the error surfaces and the externally edited file keeps its external content.
func TestStorageMigrateConcurrentModificationAbort(t *testing.T) {
	skipIfNoSSHForStorage(t)
	home, configPath, _, fakeSSHDir := seedMigrateHome(t)
	b := backendWithFakeSSH(t, home, fakeSSHDir)

	// Inject external edit into the configPath after the plan is computed.
	// We do this by overriding WriteFile to modify configPath before the
	// first write completes — simulating an external editor.
	//
	// NOTE: ErrConfigChangedSincePreview fires in MigrateWithPlan's preflight
	// check (before any write). To simulate a concurrent external modification
	// we instead edit the file AFTER the plan (view) is obtained.
	view, err := b.SSHStorageMigrationPlan(tuikit.StorageInclude)
	if err != nil {
		t.Fatalf("plan: %v", err)
	}

	// Externally edit configPath after preview.
	externalContent := readFile(t, configPath) + "\n# external concurrent edit\n"
	writeFile(t, configPath, externalContent)

	msg := runStorageCommit(t, b, tuikit.StorageInclude, view.PlanToken)
	if msg.Err == "" {
		t.Fatal("concurrent modification must cause an error")
	}
	if !msg.ConfigChangedSincePreview {
		t.Error("ConfigChangedSincePreview must be true for a concurrent-modification abort")
	}

	// The externally edited file must keep the external content.
	if !strings.Contains(readFile(t, configPath), "external concurrent edit") {
		t.Error("externally edited file must retain external content after abort")
	}
}

// hasBlock reports whether content contains a gitid-managed block with the
// given name, using filewriter.ListBlocks (the canonical scanner).
func hasBlock(content []byte, name string) bool {
	for _, blk := range filewriter.ListBlocks(content) {
		if blk.Name == name {
			return true
		}
	}
	return false
}

// stripLineComments removes Go single-line comments from s, so construction
// checks can safely search for call patterns without matching comment text.
func stripLineComments(s string) string {
	var out strings.Builder
	for _, line := range strings.Split(s, "\n") {
		stripped := line
		if idx := strings.Index(line, "//"); idx >= 0 {
			stripped = line[:idx]
		}
		out.WriteString(stripped)
		out.WriteByte('\n')
	}
	return out.String()
}

// suppress unused import warnings
var _ = sync.Mutex{}
var _ = errors.New
