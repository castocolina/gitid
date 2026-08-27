//go:build e2e

package e2e

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const (
	globalGitBaselineMarker = "# BEGIN gitid managed: global-git"
	globalGitAuthorMarker   = "# BEGIN gitid managed: global-git-author"
)

func seedGlobalGitHome(t *testing.T, home, mainConfig, baseline string) (string, string) {
	t.Helper()
	fragmentDir := filepath.Join(home, ".gitconfig.d")
	if err := os.MkdirAll(fragmentDir, 0o700); err != nil {
		t.Fatalf("creating git config directory: %v", err)
	}
	mainPath := filepath.Join(home, ".gitconfig")
	baselinePath := filepath.Join(fragmentDir, "00-baseline")
	if mainConfig != "" {
		writeFileT(t, mainPath, mainConfig)
	}
	if baseline != "" {
		writeFileT(t, baselinePath, baseline)
	}
	return mainPath, baselinePath
}

func newGlobalGitCmd(ctx context.Context, bin, home, fakeGitDir string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, bin)
	env := append(os.Environ(), "HOME="+home, "TERM=xterm-256color")
	if fakeGitDir != "" {
		env = append(env, "PATH="+fakeGitDir+":"+os.Getenv("PATH"))
	}
	cmd.Env = env
	return cmd
}

func startGlobalGitPTY(t *testing.T, home, fakeGitDir string) *ptySession {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	t.Cleanup(cancel)
	s := startPTYAt(t, newGlobalGitCmd(ctx, BuildBinary(t), home, fakeGitDir), dummyTermWidth, dummyTermHeight)
	t.Cleanup(func() { s.close(t) })
	uiReady(t, s)
	s.sendKey([]byte("3"), keystrokeDelay)
	mustSee(t, s, "init.defaultBranch", "Global Git options opens")
	return s
}

func captureGlobalGitFrame(t *testing.T, name string, s *ptySession) string {
	t.Helper()
	frame := s.snapshot()
	path := filepath.Join(repoRoot(t), ".planning", "phases", "07-global-git-options", "ui-frames", name+".txt")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("creating phase frame directory: %v", err)
	}
	if err := os.WriteFile(path, []byte(frame), 0o644); err != nil {
		t.Fatalf("writing phase frame: %v", err)
	}
	return frame
}

func moveGlobalGitRow(t *testing.T, s *ptySession, count int) {
	t.Helper()
	for range count {
		s.sendKey(dummyKeyDown, keystrokeDelay)
	}
}

func openGlobalGitPreview(t *testing.T, s *ptySession, row int) {
	t.Helper()
	moveGlobalGitRow(t, s, row)
	s.sendKey([]byte(" "), keystrokeDelay)
	mustSee(t, s, "apply 1 selected", "selecting global Git option enables apply")
	s.sendKey([]byte("a"), keystrokeDelay)
	mustSee(t, s, "Write global-git managed block to", "apply opens Global Git preview")
}

func snapshotGlobalGitFiles(t *testing.T, paths ...string) map[string][]byte {
	t.Helper()
	return snapshotGitBytes(t, paths)
}

func assertGlobalGitFilesUnchanged(t *testing.T, before map[string][]byte, paths ...string) {
	t.Helper()
	assertGitBytesUnchanged(t, before, snapshotGlobalGitFiles(t, paths...))
}

func TestGlobalGit_RealPTYBrowse(t *testing.T) {
	home := ShortSandboxHome(t)
	seedGlobalGitHome(t, home, "[init]\n\tdefaultBranch = trunk\n", "")
	s := startGlobalGitPTY(t, home, "")
	frame := captureGlobalGitFrame(t, "global-git-browse", s)
	for _, want := range []string{"init.defaultBranch", "trunk", "core.ignorecase", "user.email (global fallback)", "merge.conflictstyle"} {
		if !strings.Contains(frame, want) {
			t.Fatalf("browse frame missing %q:\n%s", want, frame)
		}
	}
	if strings.Contains(frame, "DEMO DATA") {
		t.Fatalf("Global Git carries a demo banner:\n%s", frame)
	}
}

// TestGlobalGit_RealPTYScrollBothDirections proves the boundary is stable
// under the REAL 12-row D-08 policy table: gitVisibleRowCount deliberately
// computes its budget from the canonical minFrameHeight (30, matching
// dummyTermWidth/dummyTermHeight — 07-UI-SPEC.md's frozen "overflow" row),
// and 12 rows × 2 lines = 24 always fits inside that budget (24-25
// depending on the findings banner) — so no real scroll cue can ever
// appear against the live 12-row fixture; the click-offset-mapping and cue
// mechanics themselves are proven separately in
// internal/tuikit/globalgit_test.go against an inflated stub row count
// (the only way to force real overflow without violating the frozen frame
// size or the frozen policy table). This PTY case instead proves the two
// things a REAL binary run can actually observe: no cue renders at either
// boundary, and moving to the last row and back to the first is stable.
func TestGlobalGit_RealPTYScrollBothDirections(t *testing.T) {
	home := SandboxHome(t)
	seedGlobalGitHome(t, home, "", "")
	s := startGlobalGitPTY(t, home, "")
	moveGlobalGitRow(t, s, 11)
	down := captureGlobalGitFrame(t, "global-git-scroll-down", s)
	if strings.Contains(down, "↑ (+") || strings.Contains(down, "↓ (+") {
		t.Fatalf("real 12-row fixture fits the frozen budget and must show no scroll cue:\n%s", down)
	}
	if !strings.Contains(down, "diff.colorMoved") {
		t.Fatalf("last row not reachable by keyboard navigation:\n%s", down)
	}
	for range 11 {
		s.sendKey([]byte{0x1b, 0x5b, 0x41}, keystrokeDelay)
	}
	up := captureGlobalGitFrame(t, "global-git-scroll-up", s)
	if strings.Contains(up, "↑ (+") || strings.Contains(up, "↓ (+") {
		t.Fatalf("returning to the top must still show no scroll cue:\n%s", up)
	}
	if !strings.Contains(up, "init.defaultBranch") {
		t.Fatalf("first row not reachable after returning from the bottom:\n%s", up)
	}
}

func TestGlobalGit_RealPTYEmptySelectionGuard(t *testing.T) {
	home := SandboxHome(t)
	seedGlobalGitHome(t, home, "", "")
	s := startGlobalGitPTY(t, home, "")
	before := s.snapshot()
	s.sendKey([]byte("a"), keystrokeDelay)
	after := captureGlobalGitFrame(t, "global-git-empty-selection", s)
	if after != before {
		t.Fatalf("apply with no selected options changed the screen:\nbefore:\n%s\nafter:\n%s", before, after)
	}
}

func TestGlobalGit_RealPTYApplyCancel(t *testing.T) {
	home := SandboxHome(t)
	main, baseline := seedGlobalGitHome(t, home, "[user]\n\tname = Test User\n", "")
	before := snapshotGlobalGitFiles(t, main, baseline)
	s := startGlobalGitPTY(t, home, "")
	openGlobalGitPreview(t, s, 0)
	s.sendKey(dummyKeyEsc, keystrokeDelay)
	mustSee(t, s, "init.defaultBranch", "cancel returns to Global Git options")
	captureGlobalGitFrame(t, "global-git-apply-cancel", s)
	assertGlobalGitFilesUnchanged(t, before, main, baseline)
}

func TestGlobalGit_RealPTYApplyConfirm(t *testing.T) {
	home := SandboxHome(t)
	main, baseline := seedGlobalGitHome(t, home, "[user]\n\tname = Test User\n", "")
	s := startGlobalGitPTY(t, home, "")
	openGlobalGitPreview(t, s, 0)
	s.sendKey(dummyKeyEnter, keystrokeDelay)
	mustSee(t, s, "Wrote →", "confirm writes Global Git managed block")
	mustSee(t, s, "Backed up →", "confirm reports backup")
	captureGlobalGitFrame(t, "global-git-apply-confirm", s)
	content := readFileE2E(t, baseline)
	for _, want := range []string{globalGitBaselineMarker, "defaultBranch = main", "# END gitid managed: global-git"} {
		if !strings.Contains(content, want) {
			t.Fatalf("written baseline missing %q:\n%s", want, content)
		}
	}
	backups, err := filepath.Glob(main + ".bak.*")
	if err != nil || len(backups) != 1 {
		t.Fatalf("main config backups = %v, %v; want one", backups, err)
	}
}

func TestGlobalGit_RealPTYDiffersRow(t *testing.T) {
	home := SandboxHome(t)
	seedGlobalGitHome(t, home, "[init]\n\tdefaultBranch = master\n", "")
	s := startGlobalGitPTY(t, home, "")
	// "differs" is the truncation-safe substring: the master row's own
	// column width clips the full "set, differs from recommendation"
	// sentence (the same truncation this project's unit tests already
	// route around — see TestGlobalGitDiffersRowRendersWordNotNewGlyph).
	mustSee(t, s, "differs", "conflicting user value renders differs state")
	before := s.snapshot()
	s.sendKey([]byte(" "), keystrokeDelay)
	s.sendKey([]byte("a"), keystrokeDelay)
	after := captureGlobalGitFrame(t, "global-git-differs", s)
	if after != before || strings.Contains(after, "Write global-git managed block to") {
		t.Fatalf("differs row became selectable:\n%s", after)
	}
}

// startGlobalGitPTYExpectingProbeFailure opens the Global Git tab WITHOUT
// waiting for "init.defaultBranch" (startGlobalGitPTY's success wait) — the
// whole point of this scenario is that the probe fails before any option
// row can render, so that wait would time out and never reflect the real
// failure state.
func startGlobalGitPTYExpectingProbeFailure(t *testing.T, home, fakeGitDir string) *ptySession {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	t.Cleanup(cancel)
	s := startPTYAt(t, newGlobalGitCmd(ctx, BuildBinary(t), home, fakeGitDir), dummyTermWidth, dummyTermHeight)
	t.Cleanup(func() { s.close(t) })
	uiReady(t, s)
	s.sendKey([]byte("3"), keystrokeDelay)
	return s
}

func TestGlobalGit_RealPTYProbeFailureStaysNavigable(t *testing.T) {
	home := SandboxHome(t)
	seedGlobalGitHome(t, home, "", "")
	s := startGlobalGitPTYExpectingProbeFailure(t, home, FakeGitShimDir(t, "2.50.0", "config"))
	mustSee(t, s, "git probe failed:", "probe failure names git probe")
	mustSee(t, s, "The option states could not be read from this machine.", "probe failure advisory renders")
	frame := captureGlobalGitFrame(t, "global-git-probe-failure", s)
	if strings.Contains(frame, "init.defaultBranch") {
		t.Fatalf("probe failure left option rows visible:\n%s", frame)
	}
	s.sendKey([]byte{0x1b, 0x5b, 0x43}, keystrokeDelay)
	mustSee(t, s, "Doctor", "plain right arrow leaves failed Global Git screen")
}

func TestGlobalGit_RealPTYFallbackPairSet(t *testing.T) {
	home := SandboxHome(t)
	main, _ := seedGlobalGitHome(t, home, "", "")
	s := startGlobalGitPTY(t, home, "")
	moveGlobalGitRow(t, s, 3)
	s.sendKey(dummyKeyEnter, keystrokeDelay)
	for _, r := range "Fallback User" {
		s.sendKey([]byte(string(r)), keystrokeDelay)
	}
	s.sendKey([]byte("\t"), keystrokeDelay)
	for _, r := range "fallback@example.com" {
		s.sendKey([]byte(string(r)), keystrokeDelay)
	}
	s.sendKey(dummyKeyEnter, keystrokeDelay)
	s.sendKey([]byte("a"), keystrokeDelay)
	mustSee(t, s, "Set global fallback", "fallback set opens its ceremony")
	s.sendKey(dummyKeyEnter, keystrokeDelay)
	mustSee(t, s, "Wrote →", "fallback set reaches receipt")
	captureGlobalGitFrame(t, "global-git-fallback-set", s)
	content := readFileE2E(t, main)
	for _, want := range []string{globalGitAuthorMarker, "name = Fallback User", "email = fallback@example.com"} {
		if !strings.Contains(content, want) {
			t.Fatalf("fallback config missing %q:\n%s", want, content)
		}
	}
}

func TestGlobalGit_RealPTYFallbackPairClear(t *testing.T) {
	home := SandboxHome(t)
	main, _ := seedGlobalGitHome(t, home, globalGitAuthorMarker+"\n[user]\n\tname = Old User\n\temail = old@example.com\n# END gitid managed: global-git-author\n", "")
	s := startGlobalGitPTY(t, home, "")
	moveGlobalGitRow(t, s, 3)
	s.sendKey(dummyKeyEnter, keystrokeDelay)
	for range "Old User" {
		s.sendKey(dummyKeyBackspace, keystrokeDelay)
	}
	s.sendKey([]byte("\t"), keystrokeDelay)
	for range "old@example.com" {
		s.sendKey(dummyKeyBackspace, keystrokeDelay)
	}
	s.sendKey(dummyKeyEnter, keystrokeDelay)
	s.sendKey([]byte("a"), keystrokeDelay)
	mustSee(t, s, "Remove global fallback author", "empty existing fallback opens removal ceremony")
	s.sendKey(dummyKeyEnter, keystrokeDelay)
	mustSee(t, s, "Wrote →", "fallback removal reaches receipt")
	captureGlobalGitFrame(t, "global-git-fallback-clear", s)
	if content := readFileE2E(t, main); strings.Contains(content, globalGitAuthorMarker) {
		t.Fatalf("fallback author block remains after removal:\n%s", content)
	}
}

func TestGlobalGit_RealPTYCrossWarning(t *testing.T) {
	home := SandboxHome(t)
	seedGlobalGitHome(t, home, globalGitAuthorMarker+"\n[user]\n\temail = half@example.com\n# END gitid managed: global-git-author\n", "")
	s := startGlobalGitPTY(t, home, "")
	openGlobalGitPreview(t, s, 4)
	frame := captureGlobalGitFrame(t, "global-git-cross-warning", s)
	if !strings.Contains(frame, "fallback author has no name set") {
		t.Fatalf("cross warning missing from ceremony:\n%s", frame)
	}
	s.sendKey(dummyKeyEsc, keystrokeDelay)
}

func TestGlobalGit_RealPTYBelowGateVersion(t *testing.T) {
	home := SandboxHome(t)
	_, baseline := seedGlobalGitHome(t, home, "", "")
	s := startGlobalGitPTY(t, home, FakeGitShimDir(t, "2.34.1", ""))
	openGlobalGitPreview(t, s, 10)
	s.sendKey(dummyKeyEnter, keystrokeDelay)
	mustSee(t, s, "Wrote →", "below-gate conflict style writes")
	captureGlobalGitFrame(t, "global-git-version-gate", s)
	content := readFileE2E(t, baseline)
	if !strings.Contains(content, "conflictstyle = diff3") || strings.Contains(content, "conflictstyle = zdiff3") {
		t.Fatalf("below-gate write did not use diff3:\n%s", content)
	}
}

// TestGlobalGit_RealPTYMidTransactionFailureAndRetry proves
// runGlobalGitApply's rollback-and-retry contract with a REAL filesystem
// failure — not the test-only failCommitAt hook.
//
// DEVIATION from the plan's suggested "chmod ~/.gitconfig.d to 0500"
// mechanism (recorded here per this project's "record explicitly, never
// fake" rule): runGlobalGitApply calls filewriter.EnsureDir(baselineDir,
// 0o700) immediately before writing the baseline file — EnsureDir
// unconditionally os.Chmod()s the directory back to 0700 as part of
// "ensuring" it (internal/filewriter/filewriter.go's EnsureDir), which is a
// legitimate defensive property, not a bug. That self-heal was verified
// empirically: a chmod-0500 attempt run against the real binary still
// produced "Wrote → ~/.gitconfig.d/00-baseline" — the directory permission
// mechanism cannot reach a real write-time failure from an unprivileged
// test process against this codebase.
//
// The mechanism used instead: pre-create ~/.gitconfig.d/00-baseline as a
// DIRECTORY (a real, ordinary filesystem object sitting where a regular
// file belongs) before the transaction starts. mutationJournal.watchFile
// (cmd/gitid/wiring.go) explicitly rejects any non-regular watch target
// ("gitid: refusing non-regular transaction target") — a real stat-based
// check, not a hook — so the apply fails closed before either write lands.
// Removing the bogus directory and retrying then proves the SAME ceremony
// failure → "Retry (Enter)" → success path the plan asked for, just
// triggered by a real pre-existing filesystem obstruction rather than a
// mid-write permission race that this codebase's own defensive EnsureDir
// call makes unreachable.
func TestGlobalGit_RealPTYMidTransactionFailureAndRetry(t *testing.T) {
	home := SandboxHome(t)
	main, baseline := seedGlobalGitHome(t, home, "", "")
	before := snapshotGlobalGitFiles(t, main)

	// The baseline path must not exist yet when the screen opens — the
	// initial probe (globalgit.Statuses) also os.ReadFile()s it, so seeding
	// the directory THIS early would fail the probe itself (a different,
	// earlier error) rather than exercising the write-time rejection this
	// case targets. Open the preview against a genuinely absent baseline
	// file (the same precondition TestGlobalGit_RealPTYApplyConfirm uses
	// successfully), THEN create the obstruction right before confirming.
	s := startGlobalGitPTY(t, home, "")
	openGlobalGitPreview(t, s, 0)
	mustSee(t, s, "Write global-git managed block to", "apply opens Global Git preview")

	if err := os.MkdirAll(baseline, 0o700); err != nil {
		t.Fatalf("seeding %s as a real directory (not a file) right before confirm: %v", baseline, err)
	}

	s.sendKey(dummyKeyEnter, keystrokeDelay) // confirm-write -> the real transaction rejects the non-regular target
	mustSee(t, s, "✗", "result-failure: the ceremony renders the red failure glyph")
	mustSee(t, s, "refusing non-regular transaction target", "result-failure: the exact rejection reason from wiring.go surfaces verbatim")
	mustSee(t, s, "Retry (Enter)", "result-failure: a failed write offers retry, never a silent dead end")
	failFrame := captureGlobalGitFrame(t, "global-git-mid-transaction-failure", s)
	if strings.Contains(failFrame, "Wrote →") {
		t.Fatalf("failure frame must not also claim success:\n%s", failFrame)
	}

	// Neither watched file may have been touched — the rejection happens
	// before either write is attempted.
	assertGlobalGitFilesUnchanged(t, before, main)
	if info, err := os.Stat(baseline); err != nil || !info.IsDir() {
		t.Fatalf("the bogus directory must survive the rejected transaction untouched: %v, isDir=%v", err, info != nil && info.IsDir())
	}

	if err := os.Remove(baseline); err != nil {
		t.Fatalf("clearing the bogus directory before retry: %v", err)
	}

	s.sendKey(dummyKeyEnter, keystrokeDelay) // retry -> the same transaction, now unblocked
	mustSee(t, s, "Wrote →", "retry after the obstruction is cleared succeeds")
	captureGlobalGitFrame(t, "global-git-mid-transaction-retry", s)

	content := readFileE2E(t, baseline)
	for _, want := range []string{globalGitBaselineMarker, "defaultBranch = main", "# END gitid managed: global-git"} {
		if !strings.Contains(content, want) {
			t.Fatalf("retried write missing %q:\n%s", want, content)
		}
	}
}
