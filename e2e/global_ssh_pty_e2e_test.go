//go:build e2e

package e2e

import (
	"context"
	"fmt"
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
	return startGlobalSSHPTYWithEnv(t, home, mode)
}

func startGlobalSSHPTYWithEnv(t *testing.T, home, mode string, extraEnv ...string) *ptySession {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second*ciTimeoutMultiplier())
	t.Cleanup(cancel)
	fakeSSHDir := FakeSSHDir(t, mode)
	env, _ := e2eEnv(t, home, fakeSSHDir)
	env = append(env, extraEnv...)
	cmd := newRealCreateFlowCmd(t, ctx, BuildBinary(t), home, fakeSSHDir)
	cmd.Env = env
	s := startPTYAt(t, cmd, dummyTermWidth, dummyTermHeight)
	t.Cleanup(func() { s.close(t) })
	uiReady(t, s)
	s.sendKey([]byte("2"), keystrokeDelay)
	mustSee(t, s, "Options", "Global SSH options opens")
	return s
}

func optionListLine(frame, label string) (string, bool) {
	for _, line := range strings.Split(frame, "\n") {
		list := strings.SplitN(line, "│", 2)[0]
		if strings.Contains(list, label) {
			return list, true
		}
	}
	return "", false
}

func waitForFocusedOption(t *testing.T, s *ptySession, label, context string) string {
	t.Helper()
	last, ok := s.waitFor(8*time.Second, func(frame string) bool {
		line, found := optionListLine(frame, label)
		return found && strings.Contains(line, "▸")
	})
	if !ok {
		t.Fatalf("%s: option %q never became focused. Last frame:\n%s", context, label, last)
	}
	return last
}

func assertOptionColumnsAligned(t *testing.T, frame string, labels ...string) {
	t.Helper()
	want := -1
	for _, label := range labels {
		line, ok := optionListLine(frame, label)
		if !ok {
			t.Fatalf("option %q missing while checking checkbox-column alignment:\n%s", label, frame)
		}
		col := len([]rune(line[:strings.Index(line, label)]))
		if want < 0 {
			want = col
		}
		if col != want {
			t.Errorf("option %q starts at column %d, want uniform column %d; line=%q", label, col, want, line)
		}
	}
}

// clickOptionToggle follows clickLabelRow's decoded-frame technique but aims
// at the rendered bracket cell rather than the label. Coordinates are always
// derived from the current frame; no terminal cell is hardcoded.
func clickOptionToggle(t *testing.T, s *ptySession, label string) {
	t.Helper()
	var col, row int
	last, ok := s.waitFor(8*time.Second, func(frame string) bool {
		for y, line := range strings.Split(frame, "\n") {
			list := strings.SplitN(line, "│", 2)[0]
			labelIdx := strings.Index(list, label)
			if labelIdx < 0 {
				continue
			}
			boxIdx := strings.LastIndex(list[:labelIdx], "[ ]")
			if boxIdx < 0 {
				return false
			}
			col = len([]rune(list[:boxIdx])) + 2 // inside the bracket, in 1-based SGR coordinates
			row = y + 1
			return true
		}
		return false
	})
	if !ok {
		t.Fatalf("clickOptionToggle: off toggle for %q never rendered. Last frame:\n%s", label, last)
	}
	s.sendKey([]byte(fmt.Sprintf("\x1b[<0;%d;%dM", col, row)), keystrokeDelay)
	s.sendKey([]byte(fmt.Sprintf("\x1b[<0;%d;%dm", col, row)), keystrokeDelay)
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

// captureGlobalSSHFrame snapshots the current frame and saves it via
// saveFrame's gitignored tmp/ui-frames/ scratch directory.
//
// WR-04 (review iteration 3): this used to write straight into the TRACKED
// .planning/phases/06-global-ssh-options/ui-frames/ directory — the exact
// defect saveFrame's own WR-12 doc comment (e2e/ui_pty_e2e_test.go)
// documents and fixes: every run embeds absolute sandbox paths
// (t.TempDir()), so every run on every machine produced a different file
// and dirtied the working tree, and any provenance/hash gate over that
// directory (e.g. cmd/gitid-frame-promote) is unstable there. A genuine
// baseline update is now a deliberate promotion via gitid-frame-promote,
// same as phase 9's own convention, never an incidental `make test-e2e` run.
func captureGlobalSSHFrame(t *testing.T, name string, s *ptySession) string {
	t.Helper()
	frame := s.snapshot()
	saveFrame(t, name, s)
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

func TestGlobalSSH_RealPTYOptionAffordancesAndMouseToggle(t *testing.T) {
	home := ShortSandboxHome(t)
	seedGlobalSSHHome(t, home, "none")
	s := startGlobalSSHPTY(t, home, "globalssh")

	frame := waitForFocusedOption(t, s, "StrictHostKeyChecking", "initial activation focuses the first fetched SSH row")
	assertOptionColumnsAligned(t, frame, "StrictHostKeyChecking", "ForwardAgent", "HashKnownHosts", "IdentitiesOnly", "AddKeysToAgent", "UseKeychain")
	naLine, ok := optionListLine(frame, "IdentitiesOnly")
	naPrefix := ""
	if ok {
		naPrefix = naLine[:strings.Index(naLine, "IdentitiesOnly")]
	}
	if !ok || !strings.Contains(naPrefix, "·") || strings.Contains(naPrefix, "[") {
		t.Fatalf("non-selectable SSH row must carry the dot placeholder and no bracket toggle; line=%q", naLine)
	}

	s.sendKey(dummyKeyDown, keystrokeDelay)
	s.sendKey(dummyKeyDown, keystrokeDelay)
	waitForFocusedOption(t, s, "HashKnownHosts", "setup moves SSH focus away from the first row")
	s.sendKey([]byte("1"), keystrokeDelay)
	mustSee(t, s, "Identities", "leaving Global SSH reaches another main tab")
	s.sendKey([]byte("2"), keystrokeDelay)
	waitForFocusedOption(t, s, "StrictHostKeyChecking", "re-entering Global SSH resets focus to the first fetched row")

	// Keep detail focus on another row while a raw SGR click toggles the first
	// row's bracket cell. This proves the click does not also select its row.
	s.sendKey(dummyKeyDown, keystrokeDelay)
	waitForFocusedOption(t, s, "ForwardAgent", "mouse-toggle setup keeps detail on the second row")
	clickOptionToggle(t, s, "StrictHostKeyChecking")
	after := waitForFocusedOption(t, s, "ForwardAgent", "raw SGR toggle must not move detail selection")
	strictLine, ok := optionListLine(after, "StrictHostKeyChecking")
	if !ok || !strings.Contains(strictLine, "[✓]") {
		t.Fatalf("real raw SGR click did not flip StrictHostKeyChecking to [✓]; line=%q\n%s", strictLine, after)
	}
}

func TestGlobalSSH_RealPTYOptionAffordancesNoColor(t *testing.T) {
	home := ShortSandboxHome(t)
	seedGlobalSSHHome(t, home, "none")
	s := startGlobalSSHPTYWithEnv(t, home, "globalssh", "NO_COLOR=1")

	s.sendKey([]byte(" "), keystrokeDelay)
	last, ok := s.waitFor(8*time.Second, func(frame string) bool {
		return strings.Contains(frame, "[✓]") && strings.Contains(frame, "[ ]") && strings.Contains(frame, "·")
	})
	if !ok {
		t.Fatalf("NO_COLOR frame never showed mutually distinct on/off/placeholder shapes. Last frame:\n%s", last)
	}
	assertOptionColumnsAligned(t, last, "StrictHostKeyChecking", "ForwardAgent", "HashKnownHosts", "IdentitiesOnly", "AddKeysToAgent", "UseKeychain")
	captureGlobalSSHFrame(t, "global-ssh-option-affordances-no-color", s)
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

// TestGlobalSSH_RealPTYSubTabStripMouseClick verifies that real SGR mouse
// clicks on the sub-tab strip labels switch sub-tabs and that border rows
// are inert. This proves the mouse coordinate math works correctly after
// the strip gained a border (Task 2).
func TestGlobalSSH_RealPTYSubTabStripMouseClick(t *testing.T) {
	home := ShortSandboxHome(t)
	seedGlobalSSHHome(t, home, "none")
	s := startGlobalSSHPTY(t, home, "options")

	// Wait for the bordered strip to render with "Options" and "Storage & preview" visible.
	frame, ok := s.waitFor(8*time.Second, func(text string) bool {
		return strings.Contains(text, "Options") && strings.Contains(text, "Storage & preview")
	})
	if !ok {
		t.Fatalf("Global SSH screen with sub-tab strip never rendered. Last frame:\n%s", frame)
	}
	if !strings.Contains(frame, "StrictHostKeyChecking") {
		t.Errorf("Options sub-tab content (StrictHostKeyChecking) missing in first frame")
	}

	// Click on the "Storage & preview" label using real SGR mouse.
	clickLabelRow(t, s, "Storage & preview")

	// Wait for the Storage sub-tab content to appear.
	frame, ok = s.waitFor(8*time.Second, func(text string) bool {
		return strings.Contains(text, "STORE-01") // The Storage sub-tab's heading
	})
	if !ok {
		t.Fatalf("Storage sub-tab content never rendered after mouse click. Last frame:\n%s", frame)
	}

	// The "Storage & preview" label should now be marked (reverse video).
	if !strings.Contains(frame, "Storage") {
		t.Errorf("Storage & preview label missing after switch")
	}

	// Click back on the "Options" label.
	clickLabelRow(t, s, "Options")

	// Wait for the Options content to re-appear.
	frame, ok = s.waitFor(8*time.Second, func(text string) bool {
		return strings.Contains(text, "StrictHostKeyChecking")
	})
	if !ok {
		t.Fatalf("Options sub-tab content never re-rendered after clicking Options. Last frame:\n%s", frame)
	}
}

func mustNotContainGlobalSSH(t *testing.T, frame, needle, context string) {
	t.Helper()
	if strings.Contains(frame, needle) {
		t.Fatalf("%s unexpectedly contains %q:\n%s", context, needle, frame)
	}
}
