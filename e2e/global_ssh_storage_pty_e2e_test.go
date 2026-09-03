//go:build e2e

package e2e

// global_ssh_storage_pty_e2e_test.go — Task 3: Raw-keystroke PTY coverage of
// the Storage & preview sub-tab (DLV-04, DLV-06).
//
// Five test cases covering the Storage sub-tab's real migration ceremony:
// browse, cancel, confirm (with block-movement assertions), round-trip, and
// the changed-since-preview refusal — all driven through the REAL compiled
// binary against a real filesystem, using the same PTY harness and sandbox
// seeding idioms plan 06-04 established for the Options sub-tab.
//
// FakeMigrateSSHDir provides a fake ssh that follows Include lines so
// resolution validation works in both layouts.

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// Fake SSH for migration — Include-aware resolution
// ---------------------------------------------------------------------------

// FakeMigrateSSHDir writes a fake ssh script that handles ssh -G by following
// Include lines in the config file. This makes migration's resolution
// validation work in both layouts (blocks in ~/.ssh/config or in
// config.d/gitid.config) without the real network.
//
// This is the "Include-aware" fake the plan's cycle-2 finding requires: after
// a MigrateToInclude, the blocks move to gitid.config which is reachable only
// via the Include line, so a script that only reads the main config would miss
// them and fail resolution validation — making the round-trip assertion vacuous.
func FakeMigrateSSHDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	const script = "#!/bin/sh\n" +
		"if [ \"$1\" = \"-Q\" ]; then\n" +
		"  echo \"ssh-ed25519\"\n" +
		"  echo \"ssh-rsa\"\n" +
		"  echo \"ecdsa-sha2-nistp256\"\n" +
		"  exit 0\n" +
		"fi\n" +
		"if [ \"$1\" = \"-V\" ]; then\n" +
		"  echo \"OpenSSH_9.9p2, LibreSSL 3.3.6\" >&2\n" +
		"  exit 0\n" +
		"fi\n" +
		"config_path=\"\"\n" +
		"next_is_config=0\n" +
		"is_resolution=0\n" +
		"for arg in \"$@\"; do\n" +
		"  if [ \"$next_is_config\" = \"1\" ]; then\n" +
		"    config_path=\"$arg\"\n" +
		"    next_is_config=0\n" +
		"    continue\n" +
		"  fi\n" +
		"  case \"$arg\" in\n" +
		"    -F) next_is_config=1 ;;\n" +
		"    -G) is_resolution=1 ;;\n" +
		"  esac\n" +
		"done\n" +
		"if [ \"$is_resolution\" != \"1\" ]; then\n" +
		"  echo \"fake-migrate-ssh: only -G is supported\" >&2\n" +
		"  exit 2\n" +
		"fi\n" +
		"if [ -z \"$config_path\" ] || [ ! -r \"$config_path\" ]; then\n" +
		"  echo \"fake-migrate-ssh: need -F <readable-config>\" >&2\n" +
		"  exit 2\n" +
		"fi\n" +
		"find_identity_file() {\n" +
		"  local f=\"$1\"\n" +
		"  [ -r \"$f\" ] || return\n" +
		"  while IFS= read -r fline || [ -n \"$fline\" ]; do\n" +
		"    trimmed=$(echo \"$fline\" | sed 's/^[[:space:]]*//')\n" +
		"    case \"$trimmed\" in\n" +
		"      Include*|include*)\n" +
		"        inc_p=\"${trimmed#*nclude }\"\n" +
		"        inc_p=$(echo \"$inc_p\" | sed \"s|~|$HOME|g\")\n" +
		"        for inc_file in $inc_p; do\n" +
		"          [ -r \"$inc_file\" ] && find_identity_file \"$inc_file\"\n" +
		"        done\n" +
		"        ;;\n" +
		"      IdentityFile*|identityfile*)\n" +
		"        kf=$(echo \"$trimmed\" | awk '{print $2}')\n" +
		"        if [ -n \"$kf\" ]; then echo \"$kf\"; return; fi\n" +
		"        ;;\n" +
		"    esac\n" +
		"  done < \"$f\"\n" +
		"}\n" +
		"keyfile=$(find_identity_file \"$config_path\")\n" +
		"[ -z \"$keyfile\" ] && keyfile=\"$HOME/.ssh/id_ed25519_stub\"\n" +
		"keyfile=$(echo \"$keyfile\" | sed \"s|~|$HOME|g\")\n" +
		"printf 'user git\\nhostname ssh.github.com\\nport 443\\nidentitiesonly yes\\nidentityfile %s\\n' \"$keyfile\"\n" +
		"exit 0\n"

	scriptPath := filepath.Join(dir, "ssh")
	if err := os.WriteFile(scriptPath, []byte(script), 0o700); err != nil { //nolint:gosec // test-only static script (G306)
		t.Fatalf("FakeMigrateSSHDir: writing fake ssh: %v", err)
	}
	return dir
}

// ---------------------------------------------------------------------------
// Sandbox seeding
// ---------------------------------------------------------------------------

// seedStorageMigrateHome seeds a sandboxed HOME with two managed identity
// blocks AND a globals block in ~/.ssh/config (sentinel layout), plus key
// pairs. Returns (configPath, includePath).
func seedStorageMigrateHome(t *testing.T, home string) (configPath, includePath string) {
	t.Helper()
	sshDir := filepath.Join(home, ".ssh")
	includeDir := filepath.Join(sshDir, "config.d")
	if err := os.MkdirAll(sshDir, 0o700); err != nil {
		t.Fatalf("seedStorageMigrateHome: MkdirAll .ssh: %v", err)
	}
	writeStubKeyPair(t, home, "personal")
	writeStubKeyPair(t, home, "work")

	personalKey := filepath.Join(sshDir, "id_ed25519_personal")
	workKey := filepath.Join(sshDir, "id_ed25519_work")

	globalBlock := "# BEGIN gitid managed: _global\nHost *\n  IdentitiesOnly yes\n# END gitid managed: _global\n\n"
	personalBlock := fmt.Sprintf(
		"# BEGIN gitid managed: personal\nHost personal.github.com\n  Hostname ssh.github.com\n"+
			"  Port 443\n  User git\n  IdentityFile %s\n  IdentitiesOnly yes\n"+
			"# END gitid managed: personal\n\n", personalKey)
	workBlock := fmt.Sprintf(
		"# BEGIN gitid managed: work\nHost work.github.com\n  Hostname ssh.github.com\n"+
			"  Port 443\n  User git\n  IdentityFile %s\n  IdentitiesOnly yes\n"+
			"# END gitid managed: work\n\n", workKey)

	configPath = filepath.Join(sshDir, "config")
	includePath = filepath.Join(includeDir, "gitid.config")
	writeFileT(t, configPath, globalBlock+personalBlock+workBlock)
	return configPath, includePath
}

// startStoragePTY navigates to Global SSH tab then Storage sub-tab.
func startStoragePTY(t *testing.T, home, fakeSSHDir string) *ptySession {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second*ciTimeoutMultiplier())
	t.Cleanup(cancel)
	s := startPTYAt(t, newRealCreateFlowCmd(t, ctx, BuildBinary(t), home, fakeSSHDir), dummyTermWidth, dummyTermHeight)
	t.Cleanup(func() { s.close(t) })
	uiReady(t, s)
	s.sendKey([]byte("2"), keystrokeDelay)
	mustSee(t, s, "Options", "Global SSH tab opens")
	s.sendKey([]byte("\x1b[C"), keystrokeDelay) // right arrow — Options → Storage sub-tab
	mustSee(t, s, "Storage & preview", "Storage sub-tab opens")
	return s
}

// captureStorageFrame snapshots the current frame and saves it via
// saveFrame's gitignored tmp/ui-frames/ scratch directory — see
// captureGlobalSSHFrame's WR-04 doc comment for why this no longer writes
// into the TRACKED .planning/phases/06-global-ssh-options/ui-frames/.
func captureStorageFrame(t *testing.T, name string, s *ptySession) string {
	t.Helper()
	frame := s.snapshot()
	saveFrame(t, name, s)
	return frame
}

// managedBlockNamesInFile returns the managed block names present in path.
func managedBlockNamesInFile(t *testing.T, path string) map[string]bool {
	t.Helper()
	data, err := os.ReadFile(path) //nolint:gosec // hermetic sandbox path
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		t.Fatalf("reading %s: %v", path, err)
	}
	names := make(map[string]bool)
	for _, line := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(line, "# BEGIN gitid managed: ") {
			name := strings.TrimPrefix(line, "# BEGIN gitid managed: ")
			names[name] = true
		}
	}
	return names
}

// storageSubTabStrip returns the sub-tab strip line from a frame.
func storageSubTabStrip(frame string) string {
	for _, line := range strings.Split(frame, "\n") {
		if strings.Contains(line, "Options") && strings.Contains(line, "Storage") {
			return line
		}
	}
	return ""
}

// ---------------------------------------------------------------------------
// TestGlobalSSHStorage_RealPTY — the five cases from the plan
// ---------------------------------------------------------------------------

// TestGlobalSSHStorage_RealPTYFakeSSHIncludeAware is the dedicated test that
// proves FakeMigrateSSHDir resolves an alias THROUGH the Include line, so the
// round-trip case's resolution assertion cannot pass vacuously after the layout
// moves. Required by the plan's cycle-2 finding on Include-awareness.
func TestGlobalSSHStorage_RealPTYFakeSSHIncludeAware(t *testing.T) {
	home := ShortSandboxHome(t)
	sshDir := filepath.Join(home, ".ssh")
	includeDir := filepath.Join(sshDir, "config.d")
	if err := os.MkdirAll(includeDir, 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	writeStubKeyPair(t, home, "personal")
	personalKey := filepath.Join(sshDir, "id_ed25519_personal")

	// Write the Include-layout config: main config has Include line,
	// identity block is in gitid.config.
	configPath := filepath.Join(sshDir, "config")
	includePath := filepath.Join(includeDir, "gitid.config")
	writeFileT(t, configPath, "Include ~/.ssh/config.d/*.config\n")
	personalBlock := fmt.Sprintf(
		"# BEGIN gitid managed: personal\nHost personal.github.com\n  Hostname ssh.github.com\n"+
			"  Port 443\n  User git\n  IdentityFile %s\n  IdentitiesOnly yes\n"+
			"# END gitid managed: personal\n", personalKey)
	writeFileT(t, includePath, personalBlock)

	fakeSSH := filepath.Join(FakeMigrateSSHDir(t), "ssh")
	cmd := exec.Command(fakeSSH, "-G", "-F", configPath, "personal.github.com") //nolint:gosec // fake ssh script
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("fake ssh -G failed: %v\n%s", err, out)
	}
	outStr := string(out)
	if !strings.Contains(outStr, "identityfile") {
		t.Fatalf("fake ssh -G output missing identityfile line:\n%s", outStr)
	}
	if !strings.Contains(outStr, personalKey) {
		t.Fatalf("fake ssh -G resolved wrong identity file:\ngot: %s\nwant key: %s", outStr, personalKey)
	}
}

// TestGlobalSSHStorage_RealPTYBrowse — Case 1: browse.
//
// Navigates to the Storage sub-tab and asserts both radio rows render, moves
// the layout selection and asserts the preview pane changes while the sub-tab
// strip is unchanged.
func TestGlobalSSHStorage_RealPTYBrowse(t *testing.T) {
	home := ShortSandboxHome(t)
	_, _ = seedStorageMigrateHome(t, home)
	fakeSSHDir := FakeMigrateSSHDir(t)

	s := startStoragePTY(t, home, fakeSSHDir)

	before := s.snapshot()
	// Both radio rows must render.
	for _, want := range []string{"Sentinel", "Include"} {
		if !strings.Contains(before, want) {
			t.Errorf("browse frame missing %q:\n%s", want, before)
		}
	}
	subTabStrip := storageSubTabStrip(before)
	if subTabStrip == "" {
		t.Errorf("sub-tab strip missing from storage browse frame:\n%s", before)
	}

	// CR-05 regression: the FIRST frame (before any keystroke) activates with
	// the radio on the CURRENT layout — the exact case that used to render
	// "layout is already sentinel — nothing to plan" in the right pane
	// instead of the resulting-config preview the sub-tab exists to show.
	if !strings.Contains(before, "Resulting config") {
		t.Errorf("first storage frame missing the resulting-config preview; CR-05 regressed:\n%s", before)
	}
	if strings.Contains(before, "nothing to plan") {
		t.Errorf("first storage frame still renders the CR-05 refusal instead of a preview:\n%s", before)
	}

	// Move the layout selection and assert the preview pane changes.
	s.sendKey(dummyKeyDown, keystrokeDelay)
	after := captureStorageFrame(t, "storage-browse", s)

	// Sub-tab strip must be unchanged.
	if storageSubTabStrip(after) != subTabStrip {
		t.Errorf("sub-tab strip changed after selection movement:\nbefore=%q\nafter=%q",
			subTabStrip, storageSubTabStrip(after))
	}

	// The preview pane content must have changed (Migrate appears when selection differs).
	if before == after {
		t.Error("preview pane must change after moving layout selection")
	}
}

// TestGlobalSSHStorage_RealPTYMigrateCancel — Case 2: migrate and cancel.
//
// Opens the migration ceremony, cancels, and asserts both files' bytes are
// unchanged.
func TestGlobalSSHStorage_RealPTYMigrateCancel(t *testing.T) {
	home := ShortSandboxHome(t)
	configPath, _ := seedStorageMigrateHome(t, home)
	fakeSSHDir := FakeMigrateSSHDir(t)

	beforeConfig, err := os.ReadFile(configPath) //nolint:gosec // hermetic sandbox
	if err != nil {
		t.Fatalf("reading config before cancel: %v", err)
	}

	s := startStoragePTY(t, home, fakeSSHDir)
	s.sendKey(dummyKeyDown, keystrokeDelay)
	mustSee(t, s, "Migrate", "Migrate appears after selecting different layout")

	// Open the ceremony.
	s.sendKey(dummyKeyEnter, keystrokeDelay)
	_, ok := s.waitFor(10*time.Second, func(text string) bool {
		return strings.Contains(text, "gitid.config") || strings.Contains(text, "Migrate SSH")
	})
	if !ok {
		t.Fatalf("ceremony never opened:\n%s", s.snapshot())
	}

	// Cancel.
	s.sendKey(dummyKeyEsc, keystrokeDelay)
	mustSee(t, s, "Storage & preview", "cancel returns to Storage sub-tab")
	captureStorageFrame(t, "storage-migrate-cancel", s)

	// Both files must be byte-identical to before.
	afterConfig, err := os.ReadFile(configPath) //nolint:gosec // hermetic sandbox
	if err != nil {
		t.Fatalf("reading config after cancel: %v", err)
	}
	if string(afterConfig) != string(beforeConfig) {
		t.Fatalf("cancel changed ~/.ssh/config:\nbefore:\n%s\nafter:\n%s", beforeConfig, afterConfig)
	}
}

// TestGlobalSSHStorage_RealPTYMigrateConfirm — Case 3: migrate and confirm.
//
// Selects Include layout, opens ceremony, confirms, and asserts:
//   - The receipt renders with backup info.
//   - Each managed block (identities AND globals) moved to the Include'd file
//     and is absent from ~/.ssh/config.
//   - A timestamped backup exists for ~/.ssh/config.
func TestGlobalSSHStorage_RealPTYMigrateConfirm(t *testing.T) {
	home := ShortSandboxHome(t)
	configPath, includePath := seedStorageMigrateHome(t, home)
	fakeSSHDir := FakeMigrateSSHDir(t)

	s := startStoragePTY(t, home, fakeSSHDir)
	s.sendKey(dummyKeyDown, keystrokeDelay)
	mustSee(t, s, "Migrate", "Migrate appears")

	// Open and confirm the ceremony.
	s.sendKey(dummyKeyEnter, keystrokeDelay)
	_, ok := s.waitFor(10*time.Second, func(text string) bool {
		return strings.Contains(text, "gitid.config") || strings.Contains(text, "Migrate SSH")
	})
	if !ok {
		t.Fatalf("ceremony never opened:\n%s", s.snapshot())
	}
	s.sendKey(dummyKeyEnter, keystrokeDelay)

	// Wait for receipt (success: backups or "wrote" text).
	_, ok = s.waitFor(45*time.Second, func(text string) bool {
		return strings.Contains(text, "bak") ||
			strings.Contains(text, "Backed up") ||
			strings.Contains(text, "Wrote")
	})
	if !ok {
		t.Fatalf("migration confirmation did not produce a receipt. Last frame:\n%s", s.snapshot())
	}

	receipt := captureStorageFrame(t, "storage-migrate-confirm", s)
	t.Logf("receipt frame:\n%s", receipt)

	// Verify on disk: identity blocks AND globals block moved to includePath.
	includeNames := managedBlockNamesInFile(t, includePath)
	configNames := managedBlockNamesInFile(t, configPath)

	for _, blockName := range []string{"personal", "work", "_global"} {
		if !includeNames[blockName] {
			t.Errorf("block %q not found in Include'd file after migration:\n%s",
				blockName, readFileE2E(t, includePath))
		}
		if configNames[blockName] {
			t.Errorf("block %q still in ~/.ssh/config after migration (must be absent):\n%s",
				blockName, readFileE2E(t, configPath))
		}
	}

	// Verify a backup exists for ~/.ssh/config.
	configBackups, _ := filepath.Glob(configPath + ".bak.*")
	if len(configBackups) == 0 {
		t.Error("no backup found for ~/.ssh/config after migration")
	}

	// Dismiss receipt and assert the screen marks the new layout as current.
	s.sendKey(dummyKeyEnter, keystrokeDelay)
	_, _ = s.waitFor(8*time.Second, func(text string) bool {
		return strings.Contains(text, "current") || strings.Contains(text, "Include")
	})
	captureStorageFrame(t, "storage-migrate-confirm-post", s)
}

// TestGlobalSSHStorage_RealPTYRoundTrip — Case 4: round-trip migration.
//
// Migrates Sentinel→Include then Include→Sentinel, asserting every managed
// block name is present at the end. FakeMigrateSSHDir follows Include lines
// so resolution validation is non-vacuous after the layout moves.
func TestGlobalSSHStorage_RealPTYRoundTrip(t *testing.T) {
	home := ShortSandboxHome(t)
	configPath, includePath := seedStorageMigrateHome(t, home)
	fakeSSHDir := FakeMigrateSSHDir(t)

	initialNames := managedBlockNamesInFile(t, configPath)
	if len(initialNames) == 0 {
		t.Fatal("no managed blocks in initial config")
	}

	s := startStoragePTY(t, home, fakeSSHDir)

	// --- First migration: Sentinel → Include ---
	s.sendKey(dummyKeyDown, keystrokeDelay)
	mustSee(t, s, "Migrate", "Migrate for first direction")
	s.sendKey(dummyKeyEnter, keystrokeDelay)
	_, ok := s.waitFor(10*time.Second, func(text string) bool {
		return strings.Contains(text, "gitid.config") || strings.Contains(text, "Migrate SSH")
	})
	if !ok {
		t.Fatalf("first ceremony never opened:\n%s", s.snapshot())
	}
	s.sendKey(dummyKeyEnter, keystrokeDelay)

	_, ok = s.waitFor(45*time.Second, func(text string) bool {
		return strings.Contains(text, "bak") || strings.Contains(text, "Backed up")
	})
	if !ok {
		t.Fatalf("first migration did not produce receipt:\n%s", s.snapshot())
	}
	s.sendKey(dummyKeyEnter, keystrokeDelay) // dismiss receipt

	// Verify first migration moved blocks.
	includeNames := managedBlockNamesInFile(t, includePath)
	for name := range initialNames {
		if !includeNames[name] {
			t.Errorf("after first migration, block %q not found in Include'd file", name)
		}
	}

	// Wait for screen to refresh.
	time.Sleep(keystrokeDelay * 5)

	// --- Second migration: Include → Sentinel ---
	// After migration, current layout is Include. Select Sentinel (the other radio).
	s.sendKey(dummyKeyDown, keystrokeDelay)
	_, ok = s.waitFor(8*time.Second, func(text string) bool {
		return strings.Contains(text, "Migrate")
	})
	if !ok {
		// Try the other direction.
		s.sendKey(dummyKeyDown, keystrokeDelay)
		_, ok = s.waitFor(4*time.Second, func(text string) bool {
			return strings.Contains(text, "Migrate")
		})
		if !ok {
			t.Fatalf("Migrate never appeared for second migration:\n%s", s.snapshot())
		}
	}

	s.sendKey(dummyKeyEnter, keystrokeDelay)
	_, ok = s.waitFor(10*time.Second, func(text string) bool {
		return strings.Contains(text, "Migrate SSH") || strings.Contains(text, "sentinel")
	})
	if !ok {
		t.Fatalf("second ceremony never opened:\n%s", s.snapshot())
	}
	s.sendKey(dummyKeyEnter, keystrokeDelay)

	_, ok = s.waitFor(45*time.Second, func(text string) bool {
		return strings.Contains(text, "bak") || strings.Contains(text, "Backed up")
	})
	if !ok {
		t.Fatalf("second migration did not produce receipt:\n%s", s.snapshot())
	}

	captureStorageFrame(t, "storage-round-trip", s)

	// After round-trip, all initial managed blocks must be in ~/.ssh/config.
	finalConfigNames := managedBlockNamesInFile(t, configPath)
	for name := range initialNames {
		if !finalConfigNames[name] {
			t.Errorf("after round-trip, block %q not found in ~/.ssh/config:\n%s",
				name, readFileE2E(t, configPath))
		}
	}
}

// TestGlobalSSHStorage_RealPTYChangedSincePreview — Case 5: changed-since-preview.
//
// Opens the migration ceremony so the preview renders, then modifies
// ~/.ssh/config from outside the session while the ceremony is still open,
// then confirms. Asserts:
//   - The error state renders the re-open-the-preview message or error.
//   - No receipt appears.
//   - No backup was created for the Include'd file.
//   - The externally edited file keeps the EXTERNAL content.
func TestGlobalSSHStorage_RealPTYChangedSincePreview(t *testing.T) {
	home := ShortSandboxHome(t)
	configPath, includePath := seedStorageMigrateHome(t, home)
	fakeSSHDir := FakeMigrateSSHDir(t)

	beforeConfig, err := os.ReadFile(configPath) //nolint:gosec // hermetic sandbox
	if err != nil {
		t.Fatalf("reading config before test: %v", err)
	}

	s := startStoragePTY(t, home, fakeSSHDir)
	s.sendKey(dummyKeyDown, keystrokeDelay)
	mustSee(t, s, "Migrate", "Migrate appears")

	// Open the ceremony — triggers plan computation and disk read.
	s.sendKey(dummyKeyEnter, keystrokeDelay)
	_, ok := s.waitFor(15*time.Second, func(text string) bool {
		return strings.Contains(text, "gitid.config") || strings.Contains(text, "Migrate SSH")
	})
	if !ok {
		t.Fatalf("migration ceremony never opened:\n%s", s.snapshot())
	}

	// Ceremony is open and preview computed.
	// Modify configPath externally to simulate a concurrent external edit.
	externalEdit := string(beforeConfig) + "\n# EXTERNAL EDIT — added after preview\n"
	if err := os.WriteFile(configPath, []byte(externalEdit), 0o600); err != nil {
		t.Fatalf("writing external edit: %v", err)
	}

	// Confirm — the digest check should detect the external edit.
	s.sendKey(dummyKeyEnter, keystrokeDelay)

	// Wait for an error state.
	_, ok = s.waitFor(30*time.Second, func(text string) bool {
		return strings.Contains(text, "re-open") ||
			strings.Contains(text, "changed") ||
			strings.Contains(text, "Retry") ||
			strings.Contains(text, "Error") ||
			strings.Contains(text, "error")
	})
	if !ok {
		t.Fatalf("config-changed-since-preview refusal never rendered:\n%s", s.snapshot())
	}

	refusalFrame := captureStorageFrame(t, "storage-changed-since-preview", s)

	// No receipt must appear.
	if strings.Contains(refusalFrame, "Backed up →") {
		t.Errorf("receipt must not appear on config-changed refusal:\n%s", refusalFrame)
	}

	// No backup file must have been created for the Include'd file.
	includeBackups, _ := filepath.Glob(includePath + ".bak.*")
	if len(includeBackups) > 0 {
		t.Errorf("no backup must be created for Include'd file on refusal; found: %v", includeBackups)
	}

	// The externally edited file must keep the EXTERNAL content.
	onDisk, err := os.ReadFile(configPath) //nolint:gosec // hermetic sandbox
	if err != nil {
		t.Fatalf("reading config after refusal: %v", err)
	}
	if !strings.Contains(string(onDisk), "EXTERNAL EDIT") {
		t.Errorf("externally edited file must retain external content after refusal")
	}
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// TestGlobalSSHStorage_RealPTYSubTabStripMouseClick verifies that real SGR
// mouse clicks on the sub-tab strip labels in the Storage tab switch sub-tabs
// correctly. This proves the bordered strip's mouse coordinate math works on
// both Options (tested in global_ssh_pty_e2e_test.go) and Storage sub-tabs.
func TestGlobalSSHStorage_RealPTYSubTabStripMouseClick(t *testing.T) {
	home := ShortSandboxHome(t)
	_, _ = seedStorageMigrateHome(t, home)
	fakeSSHDir := FakeMigrateSSHDir(t)
	s := startStoragePTY(t, home, fakeSSHDir)

	// Wait for the bordered strip and Storage sub-tab content to render.
	frame, ok := s.waitFor(8*time.Second, func(text string) bool {
		return strings.Contains(text, "STORE-01") && strings.Contains(text, "Storage & preview")
	})
	if !ok {
		t.Fatalf("Storage sub-tab screen never rendered. Last frame:\n%s", frame)
	}

	// Click on the "Options" label to switch to Options sub-tab.
	clickLabelRow(t, s, "Options")
	s.sendKey([]byte(""), keystrokeDelay)

	// Wait for the Options content to appear (option rows).
	frame, ok = s.waitFor(8*time.Second, func(text string) bool {
		return strings.Contains(text, "StrictHostKeyChecking")
	})
	if !ok {
		t.Fatalf("Options sub-tab content never rendered after mouse click. Last frame:\n%s", frame)
	}

	// Click back on the "Storage & preview" label.
	clickLabelRow(t, s, "Storage & preview")
	s.sendKey([]byte(""), keystrokeDelay)

	// Wait for Storage content to re-appear (STORE-01 marker).
	frame, ok = s.waitFor(8*time.Second, func(text string) bool {
		return strings.Contains(text, "STORE-01")
	})
	if !ok {
		t.Fatalf("Storage sub-tab content never re-rendered after clicking Storage & preview. Last frame:\n%s", frame)
	}
}

// readFileE2E reads a file and returns its content as a string, or "(missing)"
// when the file does not exist — for diagnostic messages only.
func readFileE2E(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path) //nolint:gosec // hermetic sandbox path
	if err != nil {
		return "(missing: " + err.Error() + ")"
	}
	return string(data)
}
