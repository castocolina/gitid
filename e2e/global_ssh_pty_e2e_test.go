//go:build e2e

package e2e

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func seedGlobalSSHHome(t *testing.T, home, placement string) string {
	t.Helper()
	sshDir := filepath.Join(home, ".ssh")
	includeDir := filepath.Join(sshDir, "config.d")
	if err := os.MkdirAll(includeDir, 0o700); err != nil {
		t.Fatalf("creating SSH config directory: %v", err)
	}
	writeStubKeyPair(t, home, "global")

	include := "Include " + filepath.Join(includeDir, "*.config") + "\n"
	shadow := "Host *\n  StrictHostKeyChecking no\n\n"
	main := include
	switch placement {
	case "above":
		main = shadow + include
	case "below":
		main = include + "\n" + shadow
	case "none":
	default:
		t.Fatalf("unknown shadow placement %q", placement)
	}
	configPath := filepath.Join(sshDir, "config")
	writeFileT(t, configPath, main)
	writeFileT(t, filepath.Join(includeDir, "gitid.config"), "Host placeholder.invalid\n  User git\n")
	return configPath
}

func startGlobalSSHPTY(t *testing.T, home, mode string) *ptySession {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	t.Cleanup(cancel)
	s := startPTYAt(t, newRealCreateFlowCmd(ctx, BuildBinary(t), home, FakeSSHDir(t, mode)), dummyTermWidth, dummyTermHeight)
	t.Cleanup(func() { s.close(t) })
	uiReady(t, s)
	s.sendKey([]byte("2"), keystrokeDelay)
	mustSee(t, s, "Options", "Global SSH options opens")
	return s
}

func openGlobalSSHPreview(t *testing.T, s *ptySession) {
	t.Helper()
	for range 3 {
		s.sendKey([]byte{0x1b, 0x5b, 0x41}, keystrokeDelay)
	}
	mustSee(t, s, "StrictHostKeyChecking", "up arrows select StrictHostKeyChecking")
	s.sendKey([]byte(" "), keystrokeDelay)
	mustSee(t, s, "apply 1 selected", "selecting StrictHostKeyChecking enables apply")
	s.sendKey([]byte("a"), keystrokeDelay)
	mustSee(t, s, "Write Host * managed block", "apply opens the one-screen preview")
}

func captureGlobalSSHFrame(t *testing.T, name string, s *ptySession) string {
	t.Helper()
	frame := s.snapshot()
	path := filepath.Join(repoRoot(t), ".planning", "phases", "06-global-ssh-options", "ui-frames", name+".txt")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("creating phase frame directory: %v", err)
	}
	if err := os.WriteFile(path, []byte(frame), 0o644); err != nil {
		t.Fatalf("writing phase frame: %v", err)
	}
	return frame
}

func TestGlobalSSH_RealPTYBrowse(t *testing.T) {
	home := ShortSandboxHome(t)
	seedGlobalSSHHome(t, home, "none")
	s := startGlobalSSHPTY(t, home, "globalssh")

	before := s.snapshot()
	for _, key := range []string{"StrictHostKeyChecking", "ForwardAgent", "HashKnownHosts", "IdentitiesOnly", "AddKeysToAgent", "UseKeychain", "Options", "Storage & preview"} {
		if !strings.Contains(before, key) {
			t.Fatalf("browse frame missing %q:\n%s", key, before)
		}
	}
	beforeStrip := globalSSHSubTabStrip(before)
	s.sendKey(dummyKeyDown, keystrokeDelay)
	mustSee(t, s, "ForwardAgent", "down arrow selects next option")
	after := captureGlobalSSHFrame(t, "global-ssh-browse", s)
	if before == after {
		t.Fatal("moving the option selection did not change the frame")
	}
	if globalSSHSubTabStrip(after) != beforeStrip {
		t.Fatalf("sub-tab strip changed after list movement:\nbefore=%q\nafter=%q", beforeStrip, globalSSHSubTabStrip(after))
	}
}

func TestGlobalSSH_RealPTYEmptySelectionGuard(t *testing.T) {
	home := SandboxHome(t)
	seedGlobalSSHHome(t, home, "none")
	s := startGlobalSSHPTY(t, home, "globalssh")

	before := s.snapshot()
	s.sendKey([]byte("a"), keystrokeDelay)
	after := captureGlobalSSHFrame(t, "global-ssh-empty-selection", s)
	if after != before {
		t.Fatalf("apply with no selected options changed the screen:\nbefore:\n%s\nafter:\n%s", before, after)
	}
}

func TestGlobalSSH_RealPTYApplyCancel(t *testing.T) {
	home := SandboxHome(t)
	configPath := seedGlobalSSHHome(t, home, "none")
	before, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("reading config before cancel: %v", err)
	}
	s := startGlobalSSHPTY(t, home, "globalssh")
	openGlobalSSHPreview(t, s)
	mustSee(t, s, "StrictHostKeyChecking accept-new", "preview shows the selected diff")
	mustSee(t, s, "~/.ssh/config.d/gitid.config", "preview names the Include-layout target")
	s.sendKey(dummyKeyEsc, keystrokeDelay)
	mustSee(t, s, "StrictHostKeyChecking", "cancel returns to options list")
	captureGlobalSSHFrame(t, "global-ssh-apply-cancel", s)
	after, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("reading config after cancel: %v", err)
	}
	if string(after) != string(before) {
		t.Fatalf("cancel changed config:\nbefore:\n%s\nafter:\n%s", before, after)
	}
}

func TestGlobalSSH_RealPTYApplyConfirm(t *testing.T) {
	home := SandboxHome(t)
	seedGlobalSSHHome(t, home, "none")
	s := startGlobalSSHPTY(t, home, "globalssh")
	openGlobalSSHPreview(t, s)
	preview := s.snapshot()
	if strings.Contains(preview, "shadow warning:") {
		t.Fatalf("clean preview unexpectedly reports shadowing:\n%s", preview)
	}
	mustNotContainGlobalSSH(t, preview, "advisory:", "clean preview")
	s.sendKey(dummyKeyEnter, keystrokeDelay)
	mustSee(t, s, "1 of", "confirm reaches receipt")
	mustSee(t, s, "Wrote →", "receipt names written target")
	mustSee(t, s, "Backed up →", "receipt names backup")
	receipt := captureGlobalSSHFrame(t, "global-ssh-apply-confirm", s)
	mustNotContainGlobalSSH(t, receipt, "shadow warning:", "clean receipt")
	mustNotContainGlobalSSH(t, receipt, "advisory:", "clean receipt")

	target := filepath.Join(home, ".ssh", "config.d", "gitid.config")
	content, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("reading written target: %v", err)
	}
	if !strings.Contains(string(content), "# BEGIN gitid managed: global-ssh") || !strings.Contains(string(content), "StrictHostKeyChecking accept-new") {
		t.Fatalf("written target misses managed directive:\n%s", content)
	}
	matches, err := filepath.Glob(target + ".bak.*")
	if err != nil {
		t.Fatalf("globbing target backups: %v", err)
	}
	if len(matches) != 1 {
		t.Fatalf("target backup count = %d, want 1 (%v)", len(matches), matches)
	}
}

func TestGlobalSSH_RealPTYShadowedFix(t *testing.T) {
	home := ShortSandboxHome(t)
	configPath := seedGlobalSSHHome(t, home, "above")
	s := startGlobalSSHPTY(t, home, "globalssh")
	openGlobalSSHPreview(t, s)
	mustSee(t, s, "shadow warning:", "main config directive above Include warns")
	mustSee(t, s, "config (line 2)", "preview names the earlier main config directive")
	captureGlobalSSHFrame(t, "global-ssh-shadowed-preview", s)
	s.sendKey(dummyKeyEnter, keystrokeDelay)
	mustSee(t, s, "advisory:", "receipt reports the still-shadowed write")
	receipt := captureGlobalSSHFrame(t, "global-ssh-shadowed-receipt", s)
	if !strings.Contains(receipt, "config (line 2)") {
		t.Fatalf("receipt does not retain source identity for %s:\n%s", configPath, receipt)
	}
}

func TestGlobalSSH_RealPTYLaterDirectiveDoesNotShadow(t *testing.T) {
	home := SandboxHome(t)
	seedGlobalSSHHome(t, home, "below")
	s := startGlobalSSHPTY(t, home, "globalssh")
	openGlobalSSHPreview(t, s)
	preview := s.snapshot()
	if strings.Contains(preview, "shadow warning:") {
		t.Fatalf("later main config directive unexpectedly reports shadowing:\n%s", preview)
	}
	s.sendKey(dummyKeyEnter, keystrokeDelay)
	mustSee(t, s, "Wrote →", "later directive case reaches receipt")
	receipt := captureGlobalSSHFrame(t, "global-ssh-later-directive", s)
	mustNotContainGlobalSSH(t, receipt, "advisory:", "main config directive below Include")
}

func TestGlobalSSH_RealPTYProbeInconclusive(t *testing.T) {
	home := SandboxHome(t)
	seedGlobalSSHHome(t, home, "none")
	s := startGlobalSSHPTY(t, home, "globalssh-inconclusive")
	openGlobalSSHPreview(t, s)
	mustSee(t, s, "simulation inconclusive", "failed isolated probe is disclosed")
	preview := captureGlobalSSHFrame(t, "global-ssh-probe-inconclusive-preview", s)
	mustNotContainGlobalSSH(t, preview, "shadow warning:", "inconclusive preview")
	s.sendKey(dummyKeyEnter, keystrokeDelay)
	mustSee(t, s, "Wrote →", "inconclusive simulation still permits backed-up apply")
	captureGlobalSSHFrame(t, "global-ssh-probe-inconclusive-receipt", s)
}

func TestGlobalSSH_RealPTYCommitFailureAndRetry(t *testing.T) {
	home := SandboxHome(t)
	configPath := seedGlobalSSHHome(t, home, "none")
	before, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("reading config before failure: %v", err)
	}
	target := filepath.Join(home, ".ssh", "config.d", "gitid.config")
	decoy := filepath.Join(home, ".ssh", "config.d", "gitid.decoy")
	seed, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("reading managed target before symlinking: %v", err)
	}
	writeFileT(t, decoy, string(seed))
	if err := os.Remove(target); err != nil {
		t.Fatalf("removing managed target before symlinking: %v", err)
	}
	if err := os.Symlink(decoy, target); err != nil {
		t.Fatalf("symlinking managed target: %v", err)
	}
	t.Cleanup(func() { _ = os.Remove(target) })

	s := startGlobalSSHPTY(t, home, "globalssh")
	openGlobalSSHPreview(t, s)
	s.sendKey(dummyKeyEnter, keystrokeDelay)
	if !strings.Contains(s.snapshot(), "Retry (Enter)") {
		t.Fatalf("write failure does not expose retry:\n%s", s.snapshot())
	}
	mustSee(t, s, "Cancel (Esc)", "write failure exposes cancel")
	failure := captureGlobalSSHFrame(t, "global-ssh-commit-failure", s)
	mustNotContainGlobalSSH(t, failure, "Wrote →", "failure receipt")
	afterFailure, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("reading config after failure: %v", err)
	}
	if string(afterFailure) != string(before) {
		t.Fatalf("failed apply changed config:\nbefore:\n%s\nafter:\n%s", before, afterFailure)
	}

	if err := os.Remove(target); err != nil {
		t.Fatalf("removing failed target symlink: %v", err)
	}
	writeFileT(t, target, string(seed))
	s.sendKey(dummyKeyEnter, keystrokeDelay)
	mustSee(t, s, "Wrote →", "retry after restoring writability succeeds")
	captureGlobalSSHFrame(t, "global-ssh-commit-retry", s)
}

func globalSSHSubTabStrip(frame string) string {
	for _, line := range strings.Split(frame, "\n") {
		if strings.Contains(line, "Options") && strings.Contains(line, "Storage & preview") {
			return line
		}
	}
	return ""
}

func mustNotContainGlobalSSH(t *testing.T, frame, needle, context string) {
	t.Helper()
	if strings.Contains(frame, needle) {
		t.Fatalf("%s unexpectedly contains %q:\n%s", context, needle, frame)
	}
}
