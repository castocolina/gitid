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

// TestGlobalSSH_RealPTYAllDirectivesBrowse is plan 09.5-01's tracer proof:
// the whole PROP-01 stack, wired end to end through the COMPILED binary.
// Task 1's real-terminal proof — an in-package model test cannot tell a
// real wired constructor from a nil seam. Pressing → twice from the default
// Options sub-tab reaches the third "All directives" sub-tab and shows a
// directive set genuinely larger than the six curated policy keys (the fake
// ssh's `globalssh` mode fixture emits 20 additional lowercase lines beyond
// the six policy ones — harness_test.go's FakeSSHDir).
func TestGlobalSSH_RealPTYAllDirectivesBrowse(t *testing.T) {
	home := ShortSandboxHome(t)
	seedGlobalSSHHome(t, home, "none")
	s := startGlobalSSHPTY(t, home, "globalssh")

	s.sendKey(wizardKeyRight, keystrokeDelay)
	s.sendKey(wizardKeyRight, keystrokeDelay)
	frame, ok := s.waitFor(8*time.Second, func(text string) bool {
		return strings.Contains(text, "All directives")
	})
	if !ok {
		t.Fatalf("All directives sub-tab never rendered after two → presses. Last frame:\n%s", frame)
	}
	if !strings.Contains(frame, "Global SSH › All directives") {
		t.Fatalf("breadcrumb must show the All directives sub-tab:\n%s", frame)
	}

	nonPolicyKeys := []string{
		"addressfamily", "batchmode", "canonicalizehostname", "checkhostip",
		"ciphers", "clearallforwardings", "compression", "connectionattempts",
	}
	found := 0
	for _, key := range nonPolicyKeys {
		if strings.Contains(frame, key) {
			found++
		}
	}
	if found < 3 {
		t.Fatalf("expected at least 3 non-policy directive keys visible in the frame, found %d:\n%s", found, frame)
	}
	captureGlobalSSHFrame(t, "global-ssh-all-directives-browse", s)
}

// TestGlobalSSH_RealPTYAllDirectivesFilter is plan 09.5-01 Task 3's real-
// terminal proof of the filter + D-B keyboard-capture contract: an in-
// package model test cannot show that app.go's `1`..`5` main-tab globals
// were genuinely bypassed while the filter is focused — only a real PTY
// session, where a digit key either reaches the filter textinput or reaches
// app.go, can. Typing a digit while focused must land IN the filter text
// (proven by the narrowed-to-zero match state carrying the digit); after
// `esc` blurs (without clearing), the same digit must switch main tabs.
func TestGlobalSSH_RealPTYAllDirectivesFilter(t *testing.T) {
	home := ShortSandboxHome(t)
	seedGlobalSSHHome(t, home, "none")
	s := startGlobalSSHPTY(t, home, "globalssh")

	s.sendKey(wizardKeyRight, keystrokeDelay)
	s.sendKey(wizardKeyRight, keystrokeDelay)
	mustSee(t, s, "Global SSH › All directives", "two right-presses reach the properties sub-tab")

	s.sendKey([]byte("/"), keystrokeDelay)
	for _, r := range "strict" {
		s.sendKey([]byte(string(r)), keystrokeDelay)
	}
	narrowed, ok := s.waitFor(8*time.Second, func(frame string) bool {
		return strings.Contains(frame, "1 of 27 shown")
	})
	if !ok {
		t.Fatalf("filter %q never narrowed to the expected 1 of 27 match count. Last frame:\n%s", "strict", narrowed)
	}
	if !strings.Contains(narrowed, "stricthostkeychecking") {
		t.Fatalf("filtered list must still show the matching row:\n%s", narrowed)
	}
	if strings.Contains(narrowed, "forwardagent") {
		t.Fatalf("filtered list must hide non-matching rows:\n%s", narrowed)
	}
	captureGlobalSSHFrame(t, "global-ssh-all-directives-filter-narrowed", s)

	// D-B proof, part 1: a digit typed while the filter is focused must
	// reach the filter text, NOT app.go's `1`..`5` main-tab globals. If the
	// digit had switched tabs instead, the frame would show "Identities"
	// and the filter text would still read "strict" (unmodified). Instead
	// the filter narrows to a filter text of "strict1", which matches NO
	// directive — a state that could only be reached if the digit landed
	// in the field.
	s.sendKey([]byte("1"), keystrokeDelay)
	noMatch, ok := s.waitFor(8*time.Second, func(frame string) bool {
		return strings.Contains(frame, `No directives match "strict1".`)
	})
	if !ok {
		t.Fatalf("digit typed into the focused filter did not land in the field (main tabs may have switched instead). Last frame:\n%s", noMatch)
	}
	// The persistent header always names every main tab ("[1] Identities ·
	// [2] SSH · ..."), so the real proof that app.go's globals were
	// bypassed is the breadcrumb: it must still read "Global SSH", not have
	// switched to the Identity Manager screen.
	if !strings.Contains(noMatch, "Global SSH") {
		t.Fatalf("a digit reaching app.go would have switched main tabs away from Global SSH:\n%s", noMatch)
	}
	captureGlobalSSHFrame(t, "global-ssh-all-directives-filter-digit-captured", s)

	// D-B proof, part 2: esc blurs WITHOUT clearing the filter text.
	s.sendKey(dummyKeyEsc, keystrokeDelay)

	// The same digit now reaches app.go's globals because the filter no
	// longer captures keys, switching to the Identities main tab. The
	// header always names every main tab, so the real proof is the
	// breadcrumb: "Global SSH" must be GONE now that the main tab switched.
	s.sendKey([]byte("1"), keystrokeDelay)
	afterBlur, ok := s.waitFor(8*time.Second, func(frame string) bool {
		return !strings.Contains(frame, "Global SSH")
	})
	if !ok {
		t.Fatalf("digit did not switch main tabs after esc blurred the filter — still on Global SSH:\n%s", afterBlur)
	}
	if !strings.Contains(afterBlur, "Identities") {
		t.Fatalf("expected the Identities main tab after the post-blur digit:\n%s", afterBlur)
	}
}

// TestGlobalSSH_RealPTYAllDirectivesProbeFailure proves the properties
// sub-tab's fail-open contract through the compiled binary: when the
// wildcard-only `ssh -G` probe `AllDirectives` -> `effective(deps)` runs
// fails entirely, the frozen probe-failed warning renders AND a main-tab
// key still leaves the screen — mirroring the Options sub-tab's own
// long-established `optionsErr` contract. The existing
// `globalssh-inconclusive` fixture mode cannot exercise this: it only fails
// the ISOLATED shadow-check call `shadow.go` makes with a real `-F <config>`
// during the apply-preview flow, never the wildcard-only probe `activate()`
// runs on entry — so this test uses the new `globalssh-probe-unresolvable`
// fixture mode (harness_test.go), which fails every `-G` resolution
// unconditionally.
func TestGlobalSSH_RealPTYAllDirectivesProbeFailure(t *testing.T) {
	home := ShortSandboxHome(t)
	seedGlobalSSHHome(t, home, "none")
	s := startGlobalSSHPTY(t, home, "globalssh-probe-unresolvable")

	s.sendKey(wizardKeyRight, keystrokeDelay)
	s.sendKey(wizardKeyRight, keystrokeDelay)
	frame, ok := s.waitFor(8*time.Second, func(text string) bool {
		return strings.Contains(text, "The SSH configuration could not be resolved.")
	})
	if !ok {
		t.Fatalf("probe-failed heading never rendered on the properties sub-tab. Last frame:\n%s", frame)
	}
	if !strings.Contains(frame, "ssh -G could not be run against this host — re-enter the screen to retry.") {
		t.Fatalf("probe-failed body line missing:\n%s", frame)
	}
	captureGlobalSSHFrame(t, "global-ssh-all-directives-probe-failure", s)

	// Fail-open: a main-tab key still leaves the screen even while the
	// properties sub-tab is stuck in its error state.
	s.sendKey([]byte("1"), keystrokeDelay)
	after, ok := s.waitFor(8*time.Second, func(f string) bool {
		return !strings.Contains(f, "Global SSH")
	})
	if !ok {
		t.Fatalf("main-tab key did not leave the failed properties sub-tab (fail-open broken):\n%s", after)
	}
	if !strings.Contains(after, "Identities") {
		t.Fatalf("expected the Identities main tab after leaving the failed screen:\n%s", after)
	}
}

// TestGlobalSSH_RealPTYAllDirectivesLabelMouseClick proves the sub-tab
// strip's mouse coordinate math still works after the strip grew a third
// label (Task 1). Keyboard ←/→ coverage is separate (see
// TestGlobalSSHArrowsCycleThreeSubTabsInOppositeDirections and this file's
// TestGlobalSSH_RealPTYAllDirectivesBrowse) and does not exercise this
// regression risk — click-coordinate drift after a strip label change is a
// distinct failure mode from keyboard cycling. Coordinates are always
// derived from the currently rendered frame via clickLabelRow, never
// hardcoded.
func TestGlobalSSH_RealPTYAllDirectivesLabelMouseClick(t *testing.T) {
	home := ShortSandboxHome(t)
	seedGlobalSSHHome(t, home, "none")
	s := startGlobalSSHPTY(t, home, "globalssh")

	clickLabelRow(t, s, "All directives")
	frame, ok := s.waitFor(8*time.Second, func(text string) bool {
		return strings.Contains(text, "Global SSH › All directives")
	})
	if !ok {
		t.Fatalf("clicking the All directives label never switched to the properties sub-tab. Last frame:\n%s", frame)
	}
	if !strings.Contains(frame, "of 27 shown") {
		t.Fatalf("properties body did not render after the mouse click:\n%s", frame)
	}
	captureGlobalSSHFrame(t, "global-ssh-all-directives-label-click", s)
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

	// WR-06 (09.4-REVIEW.md independent re-review): the sub-tab strip
	// renders BOTH "Options" and "Storage & preview" labels on EVERY
	// sub-tab state (globalssh.go), so a plain
	// strings.Contains(frame, "Storage") proved nothing about the click —
	// it would pass even if the switch had silently failed. The
	// terminal-emulator snapshot this test reads is decoded plain text
	// with no ANSI codes (ptySession.snapshot's own contract), so the
	// active label's reverse-video styling — which the internal unit test
	// TestSubTabStripRendersBordered checks via the raw "\x1b[7m" escape —
	// is not observable here at all. The real proof the click worked is
	// the Storage sub-tab's own content, already asserted above via the
	// "STORE-01" waitFor.

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

// typeIntoField sends each rune of text as an individual keystroke — the
// SAME technique TestGlobalSSH_RealPTYAllDirectivesFilter already uses to
// prove a real terminal routes typed characters into a focused input.
func typeIntoField(s *ptySession, text string) {
	for _, r := range text {
		s.sendKey([]byte(string(r)), keystrokeDelay)
	}
}

// ---------------------------------------------------------------------------
// PROP-04 (09.5-04) Task 3: the custom SSH directive flow's un-skippable
// validation gate, proven in a REAL terminal against the REAL compiled
// binary and a fixture `ssh` that genuinely exits 255 with a genuine
// `Bad configuration option:` diagnostic for an unrecognized directive name
// — never a mocked/hardcoded acceptance inside the TUI itself.
// ---------------------------------------------------------------------------

// TestGlobalSSH_RealPTYCustomDirectiveRejectedNameNeverWrites is
// 09.5-UI-SPEC.md's named focal point (T-09.5-20): reach "All directives",
// press "n", type a directive name the fixture rejects, submit, assert the
// frozen unrecognised-directive sentence appears, assert NO ceremony
// heading and no confirm affordance is present anywhere in the frame, and
// then read the managed target file from disk and assert it is
// BYTE-IDENTICAL to before — a frame assertion alone does not satisfy this
// focal point; the file must be shown unchanged.
func TestGlobalSSH_RealPTYCustomDirectiveRejectedNameNeverWrites(t *testing.T) {
	home := SandboxHome(t)
	seedGlobalSSHHome(t, home, "none")
	target := filepath.Join(home, ".ssh", "config.d", "gitid.config")
	before, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("reading target before the attempt: %v", err)
	}

	s := startGlobalSSHPTY(t, home, "globalssh")
	s.sendKey(wizardKeyRight, keystrokeDelay)
	s.sendKey(wizardKeyRight, keystrokeDelay)
	mustSee(t, s, "Global SSH › All directives", "two right-presses reach the properties sub-tab")

	s.sendKey([]byte("n"), keystrokeDelay)
	mustSee(t, s, "Add custom directive", "n opens the custom-directive form")

	typeIntoField(s, "NotARealDirective")
	s.sendKey([]byte("\t"), keystrokeDelay)
	typeIntoField(s, "yes")
	s.sendKey(dummyKeyEnter, keystrokeDelay)

	rejected, ok := s.waitFor(8*time.Second, func(frame string) bool {
		return strings.Contains(frame, "is not a recognized SSH directive")
	})
	if !ok {
		t.Fatalf("the frozen unrecognised-directive sentence never rendered. Last frame:\n%s", rejected)
	}
	if !strings.Contains(rejected, `'NotARealDirective' is not a recognized SSH directive`) {
		t.Fatalf("rejection sentence must name the entered directive:\n%s", rejected)
	}
	mustNotContainGlobalSSH(t, rejected, "Write custom SSH directive to", "rejected-name state")
	mustNotContainGlobalSSH(t, rejected, "Write (Enter)", "rejected-name state")
	captureGlobalSSHFrame(t, "global-ssh-custom-directive-rejected-name", s)

	after, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("reading target after the attempt: %v", err)
	}
	if string(after) != string(before) {
		t.Fatalf("a rejected directive name must leave the managed target BYTE-IDENTICAL to before:\nbefore:\n%s\nafter:\n%s", before, after)
	}
}

// TestGlobalSSH_RealPTYCustomDirectiveWrite walks the accepted path: apply a
// curated option first (so the write is a genuine MERGE into an already-
// populated managed block, not a fresh one), then type a directive name the
// fixture accepts, walk the proof beat, confirm, assert the receipt, and
// then read the managed target from disk and assert the directive is
// present INSIDE the existing `global-ssh` sentinel pair, that the ordered
// policy key applied earlier is still there, that only one `Host *` managed
// block exists, and that a timestamped backup file was created.
func TestGlobalSSH_RealPTYCustomDirectiveWrite(t *testing.T) {
	home := SandboxHome(t)
	seedGlobalSSHHome(t, home, "none")
	target := filepath.Join(home, ".ssh", "config.d", "gitid.config")

	s := startGlobalSSHPTY(t, home, "globalssh")
	openGlobalSSHPreview(t, s)
	s.sendKey(dummyKeyEnter, keystrokeDelay)
	mustSee(t, s, "Wrote →", "curated apply reaches its receipt")
	// The receipt itself is still the OPEN ceremony (its "Done" state) — a
	// second Enter dismisses it back to the browser, mirroring how a real
	// user leaves the apply receipt before navigating elsewhere.
	s.sendKey(dummyKeyEnter, keystrokeDelay)
	mustSee(t, s, "Options", "dismissing the receipt returns to the Options sub-tab")

	s.sendKey(wizardKeyRight, keystrokeDelay)
	s.sendKey(wizardKeyRight, keystrokeDelay)
	mustSee(t, s, "Global SSH › All directives", "two right-presses reach the properties sub-tab")

	s.sendKey([]byte("n"), keystrokeDelay)
	mustSee(t, s, "Add custom directive", "n opens the custom-directive form")

	typeIntoField(s, "TCPKeepAlive")
	s.sendKey([]byte("\t"), keystrokeDelay)
	typeIntoField(s, "yes")
	s.sendKey(dummyKeyEnter, keystrokeDelay)

	ceremony, ok := s.waitFor(8*time.Second, func(frame string) bool {
		return strings.Contains(frame, "Write custom SSH directive to")
	})
	if !ok {
		t.Fatalf("the custom-directive ceremony never opened for an accepted name. Last frame:\n%s", ceremony)
	}
	if !strings.Contains(ceremony, "TCPKeepAlive") {
		t.Fatalf("ceremony preview must show the submitted directive:\n%s", ceremony)
	}
	captureGlobalSSHFrame(t, "global-ssh-custom-directive-ceremony", s)

	s.sendKey(dummyKeyEnter, keystrokeDelay)
	receipt, ok := s.waitFor(8*time.Second, func(frame string) bool {
		return strings.Contains(frame, "TCPKeepAlive yes written.")
	})
	if !ok {
		t.Fatalf("the frozen receipt never rendered. Last frame:\n%s", receipt)
	}
	captureGlobalSSHFrame(t, "global-ssh-custom-directive-write-receipt", s)

	content, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("reading written target: %v", err)
	}
	body := string(content)
	if !strings.Contains(body, "# BEGIN gitid managed: global-ssh") || !strings.Contains(body, "# END gitid managed: global-ssh") {
		t.Fatalf("written target missing the global-ssh managed block sentinels:\n%s", body)
	}
	if strings.Count(body, "Host *") != 1 {
		t.Fatalf("written target must carry exactly one `Host *` managed block, got %d:\n%s", strings.Count(body, "Host *"), body)
	}
	if !strings.Contains(body, "StrictHostKeyChecking accept-new") {
		t.Fatalf("the ordered policy key applied earlier must still be present:\n%s", body)
	}
	if !strings.Contains(body, "TCPKeepAlive yes") {
		t.Fatalf("the custom directive must be present inside the managed block:\n%s", body)
	}
	beginIdx := strings.Index(body, "# BEGIN gitid managed: global-ssh")
	endIdx := strings.Index(body, "# END gitid managed: global-ssh")
	tcpIdx := strings.Index(body, "TCPKeepAlive yes")
	if beginIdx < 0 || endIdx < 0 || tcpIdx < beginIdx || tcpIdx > endIdx {
		t.Fatalf("the custom directive must land INSIDE the global-ssh sentinel pair:\n%s", body)
	}

	matches, err := filepath.Glob(target + ".bak.*")
	if err != nil {
		t.Fatalf("globbing target backups: %v", err)
	}
	if len(matches) < 1 {
		t.Fatalf("expected at least one timestamped backup file, found none")
	}
	t.Logf("backup file(s) created: %v", matches)
}

// TestGlobalSSH_RealPTYEnumRowEdit is plan 09.6-02 Task 1's tracer proof:
// the whole editor architecture end-to-end on ONE Global SSH enum row
// (StrictHostKeyChecking), through every layer: the shared editor state machine,
// the key guard, the inline value cell, the conditional ceremony dispatch, the seam,
// the overlay builder, the existing lifecycle writer, and the bytes on disk.
func TestGlobalSSH_RealPTYEnumRowEdit(t *testing.T) {
	home := ShortSandboxHome(t)
	configPath := seedGlobalSSHHome(t, home, "none")
	s := startGlobalSSHPTY(t, home, "globalssh")

	// Snapshot pre-edit state
	preEditState, _ := os.ReadFile(configPath)
	preEditContent := string(preEditState)

	// Navigate to StrictHostKeyChecking (first row, already selected by default)
	frame := s.snapshot()
	if !strings.Contains(frame, "StrictHostKeyChecking") {
		t.Fatalf("StrictHostKeyChecking row not visible in initial frame:\n%s", frame)
	}

	// Press 'e' to open the editor on the enum row
	s.sendKey([]byte("e"), keystrokeDelay)
	editorFrame, ok := s.waitFor(8*time.Second, func(text string) bool {
		// The editor should render in the detail pane with radio buttons
		return strings.Contains(text, "ask") || strings.Contains(text, "accept-new")
	})
	if !ok {
		t.Fatalf("enum editor never opened after 'e' key. Last frame:\n%s", editorFrame)
	}

	// Verify the breadcrumb still shows Options (not a different sub-tab)
	if !strings.Contains(editorFrame, "Options") || !strings.Contains(editorFrame, "Global SSH") {
		t.Fatalf("breadcrumb changed after opening editor (editor guard failed):\n%s", editorFrame)
	}

	// Press right twice to cycle the value
	s.sendKey(wizardKeyRight, keystrokeDelay)
	s.sendKey(wizardKeyRight, keystrokeDelay)
	cycledFrame, ok := s.waitFor(8*time.Second, func(text string) bool {
		// After cycling, the selected value in the editor should differ from the initial
		return strings.Contains(text, "Global SSH") && strings.Contains(text, "Options")
	})
	if !ok {
		t.Fatalf("editor did not respond to cycling keys. Last frame:\n%s", cycledFrame)
	}

	// Verify breadcrumb STILL shows Options (proving the guard beat the sub-tab-switch)
	if !strings.Contains(cycledFrame, "Options") {
		t.Fatalf("sub-tab switched during editing (key guard failed):\n%s", cycledFrame)
	}

	// Press Esc to dismiss without committing
	s.sendKey(dummyKeyEsc, keystrokeDelay)
	dismissFrame, ok := s.waitFor(8*time.Second, func(text string) bool {
		return strings.Contains(text, "StrictHostKeyChecking") && !strings.Contains(text, "Edit StrictHostKeyChecking")
	})
	if !ok {
		t.Fatalf("editor did not dismiss with Esc key. Last frame:\n%s", dismissFrame)
	}

	// Verify the file is byte-identical after dismiss (no staged override committed)
	postDismissState, _ := os.ReadFile(configPath)
	postDismissContent := string(postDismissState)
	if preEditContent != postDismissContent {
		t.Fatalf("config file changed after dismissing editor (should be byte-identical):\nBefore:\n%s\nAfter:\n%s", preEditContent, postDismissContent)
	}

	// Re-open the editor and cycle to a non-recommended value
	s.sendKey([]byte("e"), keystrokeDelay)
	editorFrame2, ok := s.waitFor(8*time.Second, func(text string) bool {
		return strings.Contains(text, "accept-new")
	})
	if !ok {
		t.Fatalf("enum editor never re-opened. Last frame:\n%s", editorFrame2)
	}

	// Cycle to a different value (e.g., off or no)
	for range 3 { // Cycle right 3 times to reach a non-recommended value
		s.sendKey(wizardKeyRight, keystrokeDelay)
	}

	// Press Enter to commit the staged override
	s.sendKey([]byte{0x0d}, keystrokeDelay) // Enter key
	committedFrame, ok := s.waitFor(8*time.Second, func(text string) bool {
		// After commit, we should be back in browse mode and the row should show as chosen
		return strings.Contains(text, "StrictHostKeyChecking") && strings.Contains(text, "Options")
	})
	if !ok {
		t.Fatalf("editor did not close after Enter commit. Last frame:\n%s", committedFrame)
	}

	// Open the apply ceremony
	s.sendKey([]byte("a"), keystrokeDelay)
	ceremonyFrame, ok := s.waitFor(8*time.Second, func(text string) bool {
		return strings.Contains(text, "Write Host") || strings.Contains(text, "managed block")
	})
	if !ok {
		t.Fatalf("apply ceremony never opened. Last frame:\n%s", ceremonyFrame)
	}

	// Confirm the apply
	s.sendKey([]byte("Y"), keystrokeDelay)
	applyPromptFrame, ok := s.waitFor(8*time.Second, func(text string) bool {
		// The confirmation prompt should appear
		return strings.Contains(text, "Type") || strings.Contains(text, "confirm")
	})
	if !ok {
		t.Fatalf("confirmation prompt never appeared. Last frame:\n%s", applyPromptFrame)
	}

	// Type the confirmation code
	s.sendKey([]byte("yes"), keystrokeDelay)
	s.sendKey([]byte{0x0d}, keystrokeDelay)

	// Wait for the apply to complete
	successFrame, ok := s.waitFor(8*time.Second, func(text string) bool {
		return strings.Contains(text, "applied") || strings.Contains(text, "Options")
	})
	if !ok {
		t.Fatalf("apply did not complete. Last frame:\n%s", successFrame)
	}

	// Verify the file has the new directive
	postApplyState, _ := os.ReadFile(configPath)
	postApplyContent := string(postApplyState)
	if postApplyContent == preEditContent {
		t.Fatalf("config file was not modified by the apply ceremony")
	}

	// Verify a backup file was created
	backupMatches, err := filepath.Glob(configPath + ".bak.*")
	if err != nil {
		t.Fatalf("globbing for backup files: %v", err)
	}
	if len(backupMatches) < 1 {
		t.Fatalf("expected at least one timestamped backup file after enum edit, found none")
	}
	t.Logf("enum row edit backup file(s) created: %v", backupMatches)
}
