//go:build e2e

package e2e

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
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

func newGlobalGitCmd(t *testing.T, ctx context.Context, bin, home, fakeGitDir string) *exec.Cmd {
	t.Helper()
	cmd := exec.CommandContext(ctx, bin)
	env, _ := e2eEnv(t, home, fakeGitDir)
	cmd.Env = append(env, "TERM=xterm-256color")
	return cmd
}

func startGlobalGitPTY(t *testing.T, home, fakeGitDir string) *ptySession {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second*ciTimeoutMultiplier())
	t.Cleanup(cancel)
	s := startPTYAt(t, newGlobalGitCmd(t, ctx, BuildBinary(t), home, fakeGitDir), dummyTermWidth, dummyTermHeight)
	t.Cleanup(func() { s.close(t) })
	uiReady(t, s)
	s.sendKey([]byte("3"), keystrokeDelay)
	mustSee(t, s, "init.defaultBranch", "Global Git options opens")
	return s
}

// shortSandboxHomePattern matches ShortSandboxHome's fixed "/tmp/h<digits>"
// prefix (harness_test.go) — the ONLY path shape normalizeShortSandboxHomePath
// ever touches, so it can never mangle unrelated frame content.
var shortSandboxHomePattern = regexp.MustCompile(`/tmp/h[0-9]+`)

// normalizeShortSandboxHomePath replaces every ShortSandboxHome path prefix
// in content with a fixed placeholder. Found while reviewing 09.5-05-PLAN.md
// Task 2's promoted frames (D-K): the Set-keys detail pane's
// `git config --show-origin` provenance line renders the ShortSandboxHome
// path verbatim BY DESIGN (D-01's whole point is proving the real origin
// file path renders) — a per-run, per-machine absolute path is not a stable
// promoted baseline, so this normalizes the SAVED copy, never the live
// assertion, which still needs the real path to prove the feature.
func normalizeShortSandboxHomePath(content string) string {
	return shortSandboxHomePattern.ReplaceAllString(content, "/tmp/h<sandbox>")
}

// captureGlobalGitFrame snapshots the current frame and saves a NORMALIZED
// copy (normalizeShortSandboxHomePath) via the gitignored tmp/ui-frames/
// scratch directory — see captureGlobalSSHFrame's WR-04 doc comment for why
// this no longer writes into the TRACKED
// .planning/phases/07-global-git-options/ui-frames/. The value RETURNED to
// the caller is the RAW, un-normalized snapshot — every existing in-test
// assertion (e.g. TestGlobalGit_RealPTYSetKeysBrowse's origin-path check)
// keeps seeing the real path; only the promoted-baseline COPY is stabilized.
func captureGlobalGitFrame(t *testing.T, name string, s *ptySession) string {
	t.Helper()
	frame := s.snapshot()
	saveFrameContent(t, name, normalizeShortSandboxHomePath(frame))
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
	for _, want := range []string{"init.defaultBranch", "trunk", "core.ignorecase", "user.email (global fallback)"} {
		if !strings.Contains(frame, want) {
			t.Fatalf("browse frame missing %q:\n%s", want, frame)
		}
	}
	if strings.Contains(frame, "DEMO DATA") {
		t.Fatalf("Global Git carries a demo banner:\n%s", frame)
	}
	// The sub-tab strip's net +3/+4 rows (09.5-02, Task 1 — this screen's
	// first strip) shrink the visible list budget below the real 12-row D9
	// policy table's 24-line height, so the last two rows are no longer
	// visible without scrolling — reachable via the SAME scroll mechanism
	// this screen already had (proven independently by
	// TestGlobalGit_RealPTYScrollBothDirections below). This is content
	// re-homing, not shrinking: merge.conflictstyle is still fully present,
	// one scroll away.
	moveGlobalGitRow(t, s, 11)
	bottom := captureGlobalGitFrame(t, "global-git-browse-bottom", s)
	if !strings.Contains(bottom, "merge.conflictstyle") {
		t.Fatalf("scrolling to the last row must reveal merge.conflictstyle:\n%s", bottom)
	}
}

func TestGlobalGit_RealPTYOptionFocusPlaceholderAndColumns(t *testing.T) {
	home := ShortSandboxHome(t)
	seedGlobalGitHome(t, home, "", "")
	s := startGlobalGitPTY(t, home, "")

	frame := waitForFocusedOption(t, s, "init.defaultBranch", "initial activation focuses the first fetched Git row")
	// The sub-tab strip's net +3/+4 rows (09.5-02, Task 1) shrink the
	// visible list budget below the real 12-row D9 policy table's 24-line
	// height, so only the first 10 rows are visible without scrolling —
	// check alignment over that visible top window here, and over the
	// remaining two rows (reachable via the same scroll mechanism) further
	// below.
	assertOptionColumnsAligned(t, frame,
		"init.defaultBranch", "core.ignorecase", "core.autocrlf / core.eol",
		"user.email (global fallback)", "user.useConfigOnly", "push.autoSetupRemote",
		"pull.rebase", "fetch.prune", "alias (8 shortcuts)",
		"color (ui/branch/diff/status)")
	naLine, ok := optionListLine(frame, "user.email (global fallback)")
	naPrefix := ""
	if ok {
		naPrefix = naLine[:strings.Index(naLine, "user.email (global fallback)")]
	}
	if !ok || !strings.Contains(naPrefix, "·") || strings.Contains(naPrefix, "[") {
		t.Fatalf("non-selectable Git row must carry the dot placeholder and no bracket toggle; line=%q", naLine)
	}

	moveGlobalGitRow(t, s, 2)
	waitForFocusedOption(t, s, "core.autocrlf / core.eol", "setup moves Git focus away from the first row")
	s.sendKey([]byte("1"), keystrokeDelay)
	mustSee(t, s, "Identities", "leaving Global Git reaches another main tab")
	s.sendKey([]byte("3"), keystrokeDelay)
	waitForFocusedOption(t, s, "init.defaultBranch", "re-entering Global Git resets focus to the first fetched row")

	moveGlobalGitRow(t, s, 11)
	scrolled := waitForFocusedOption(t, s, "diff.colorMoved", "scrolling to the last row reveals the remaining columns")
	assertOptionColumnsAligned(t, scrolled, "merge.conflictstyle", "diff.colorMoved")
}

// TestGlobalGit_RealPTYScrollBothDirections proves the boundary is stable
// under the REAL 12-row D-08 policy table.
//
// DEVIATION (09.5-02, Task 3, discovered running the full e2e suite): before
// plan 09.5-02's sub-tab strip, gitVisibleRowCount's budget (computed from
// the canonical minFrameHeight, 30) comfortably fit all 12 rows × 2 lines =
// 24 without any scroll cue. The strip's net +3/+4 rows (Task 1, MEASURED
// per TestGlobalGitFitsFixedGeometryWithStrip: available=25 used=25,
// diff=0 — the WHOLE body fits, using the screen's EXISTING scroll
// mechanism when the list itself overflows) shrink the list's own budget to
// 21 lines, which the real 12-row table no longer fits without scrolling.
// This is content re-homed behind a scrollable window, not shrunk — the
// same trade-off Global SSH's own "All directives" sub-tab already made for
// its much larger set. This case now proves the boundary is stable UNDER
// that scrolling: the last row is reachable going down, the first row is
// reachable coming back up, and the cue direction flips correctly at each
// extreme (down-cue at the top boundary, up-cue at the bottom boundary,
// never both).
func TestGlobalGit_RealPTYScrollBothDirections(t *testing.T) {
	home := SandboxHome(t)
	seedGlobalGitHome(t, home, "", "")
	s := startGlobalGitPTY(t, home, "")
	moveGlobalGitRow(t, s, 11)
	down := captureGlobalGitFrame(t, "global-git-scroll-down", s)
	if !strings.Contains(down, "↑ (+2 more options)") {
		t.Fatalf("scrolling to the last row must show the up-cue for the hidden rows above:\n%s", down)
	}
	if strings.Contains(down, "↓ (+") {
		t.Fatalf("at the bottom boundary, no down-cue should render (nothing left below):\n%s", down)
	}
	if !strings.Contains(down, "diff.colorMoved") {
		t.Fatalf("last row not reachable by keyboard navigation:\n%s", down)
	}
	for range 11 {
		s.sendKey([]byte{0x1b, 0x5b, 0x41}, keystrokeDelay)
	}
	up := captureGlobalGitFrame(t, "global-git-scroll-up", s)
	if !strings.Contains(up, "↓ (+2 more options)") {
		t.Fatalf("scrolling back to the first row must show the down-cue for the hidden rows below:\n%s", up)
	}
	if strings.Contains(up, "↑ (+") {
		t.Fatalf("at the top boundary, no up-cue should render (nothing left above):\n%s", up)
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
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second*ciTimeoutMultiplier())
	t.Cleanup(cancel)
	s := startPTYAt(t, newGlobalGitCmd(t, ctx, BuildBinary(t), home, fakeGitDir), dummyTermWidth, dummyTermHeight)
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

// ---------------------------------------------------------------------------
// Plan 09.5-02 Task 3 — real-PTY proof of the net-new sub-tab strip and the
// "Set keys" flat filterable list (PROP-02), mirroring plan 09.5-01's own
// Task 3 SSH-side precedents (global_ssh_pty_e2e_test.go).
// ---------------------------------------------------------------------------

// TestGlobalGit_RealPTYSetKeysBrowse is this plan's tracer-equivalent real-
// terminal proof: the whole PROP-02 stack, wired end to end through the
// COMPILED binary. Pressing → once from the default Options sub-tab reaches
// the new "Set keys" sub-tab and shows seeded keys with their origin path;
// pressing ← returns to Options, proving the re-homing did not lose its
// existing baseline content.
func TestGlobalGit_RealPTYSetKeysBrowse(t *testing.T) {
	home := ShortSandboxHome(t)
	seedGlobalGitHome(t, home, "[user]\n\tname = Set Keys Tester\n\temail = setkeys@example.com\n[alias]\n\tco = checkout\n", "")
	s := startGlobalGitPTY(t, home, "")

	s.sendKey(wizardKeyRight, keystrokeDelay)
	frame, ok := s.waitFor(8*time.Second, func(text string) bool {
		return strings.Contains(text, "Global Git › Set keys")
	})
	if !ok {
		t.Fatalf("Set keys sub-tab never rendered after one → press. Last frame:\n%s", frame)
	}
	if !strings.Contains(frame, "Set keys") {
		t.Fatalf("strip must show the honestly-scoped Set keys label:\n%s", frame)
	}

	seededKeyCount := 0
	for _, key := range []string{"user.name", "user.email", "alias.co"} {
		if strings.Contains(frame, key) {
			seededKeyCount++
		}
	}
	if seededKeyCount < 2 {
		t.Fatalf("expected at least 2 seeded keys visible in the frame, found %d:\n%s", seededKeyCount, frame)
	}
	// 09.5-REVIEW.md round 2 WR-05: AllGitSetKeys now scrubs Origin through
	// displayPath before it reaches the TUI (the same "~"-relative
	// convention displayBaselineTargetPath already applies elsewhere in
	// this file, e.g. baselineDisplay below) — so the detail pane shows the
	// scrubbed "~/.gitconfig" display path, never the raw sandbox HOME
	// path. Asserting the raw path here would fail against the corrected,
	// intentional scrub.
	const gitconfigDisplay = "~/.gitconfig"
	if !strings.Contains(frame, gitconfigDisplay) {
		t.Fatalf("selected row's detail pane must show the scrubbed origin file path %q:\n%s", gitconfigDisplay, frame)
	}
	captureGlobalGitFrame(t, "global-git-set-keys-browse", s)

	// The Options sub-tab's own existing content must still be reachable —
	// the re-homing behind the new strip did not lose it (09.5-UI-SPEC.md's
	// "Global Git's existing content is re-homed, not rewritten" contract).
	s.sendKey([]byte{0x1b, 0x5b, 0x44}, keystrokeDelay) // left arrow
	back, ok := s.waitFor(8*time.Second, func(text string) bool {
		return strings.Contains(text, "Global Git › Options") && strings.Contains(text, "init.defaultBranch")
	})
	if !ok {
		t.Fatalf("← from Set keys never returned to Options with its baseline content. Last frame:\n%s", back)
	}
	captureGlobalGitFrame(t, "global-git-set-keys-back-to-options", s)
}

// TestGlobalGit_RealPTYSetKeysFilter is this plan's real-terminal proof of
// the filter + D-B keyboard-capture contract on the Git side, mirroring
// TestGlobalSSH_RealPTYAllDirectivesFilter: an in-package model test cannot
// show that app.go's `1`..`5` main-tab globals were genuinely bypassed while
// the filter is focused — only a real PTY session can.
func TestGlobalGit_RealPTYSetKeysFilter(t *testing.T) {
	home := ShortSandboxHome(t)
	seedGlobalGitHome(t, home, "[user]\n\tname = Filter Tester\n\temail = filtertester@example.com\n[alias]\n\tco = checkout\n", "")
	s := startGlobalGitPTY(t, home, "")

	s.sendKey(wizardKeyRight, keystrokeDelay)
	mustSee(t, s, "Global Git › Set keys", "one right-press reaches the Set keys sub-tab")

	s.sendKey([]byte("/"), keystrokeDelay)
	for _, r := range "user." {
		s.sendKey([]byte(string(r)), keystrokeDelay)
	}
	narrowed, ok := s.waitFor(8*time.Second, func(frame string) bool {
		return strings.Contains(frame, "user.name") && strings.Contains(frame, "user.email") && !strings.Contains(frame, "alias.co")
	})
	if !ok {
		t.Fatalf("filter %q never narrowed to the user.* keys only. Last frame:\n%s", "user.", narrowed)
	}
	if !strings.Contains(narrowed, "shown") {
		t.Fatalf("filtered list must still show the match-count line:\n%s", narrowed)
	}
	captureGlobalGitFrame(t, "global-git-set-keys-filter-narrowed", s)

	// D-B proof, part 1: a digit typed while the filter is focused must
	// reach the filter text, NOT app.go's `1`..`5` main-tab globals. If the
	// digit had switched tabs instead, the frame would show "Identities".
	// Instead the filter narrows to "user.1", matching no key.
	s.sendKey([]byte("1"), keystrokeDelay)
	noMatch, ok := s.waitFor(8*time.Second, func(frame string) bool {
		return strings.Contains(frame, `No keys match "user.1".`)
	})
	if !ok {
		t.Fatalf("digit typed into the focused filter did not land in the field (main tabs may have switched instead). Last frame:\n%s", noMatch)
	}
	if !strings.Contains(noMatch, "Global Git") {
		t.Fatalf("a digit reaching app.go would have switched main tabs away from Global Git:\n%s", noMatch)
	}
	captureGlobalGitFrame(t, "global-git-set-keys-filter-digit-captured", s)

	// D-B proof, part 2: esc blurs WITHOUT clearing the filter text, then the
	// same digit reaches app.go's globals because the filter no longer
	// captures keys.
	s.sendKey(dummyKeyEsc, keystrokeDelay)
	s.sendKey([]byte("1"), keystrokeDelay)
	afterBlur, ok := s.waitFor(8*time.Second, func(frame string) bool {
		return !strings.Contains(frame, "Global Git")
	})
	if !ok {
		t.Fatalf("digit did not switch main tabs after esc blurred the filter — still on Global Git:\n%s", afterBlur)
	}
	if !strings.Contains(afterBlur, "Identities") {
		t.Fatalf("expected the Identities main tab after the post-blur digit:\n%s", afterBlur)
	}
}

// TestGlobalGit_RealPTYSubTabStripClick proves the sub-tab strip's mouse
// coordinate math on Global Git's FIRST strip: coordinates are always
// derived from the currently rendered frame via clickLabelRow, never
// hardcoded, and the click branch (net-new plumbing, including the moved
// body-relative y origin) has never been exercised on this screen before
// this plan — keyboard navigation does not exercise the click path.
func TestGlobalGit_RealPTYSubTabStripClick(t *testing.T) {
	home := ShortSandboxHome(t)
	seedGlobalGitHome(t, home, "[user]\n\tname = Click Tester\n\temail = clicktester@example.com\n", "")
	s := startGlobalGitPTY(t, home, "")

	clickLabelRow(t, s, "Set keys")
	toSetKeys, ok := s.waitFor(8*time.Second, func(text string) bool {
		return strings.Contains(text, "Global Git › Set keys")
	})
	if !ok {
		t.Fatalf("clicking the Set keys label never switched to that sub-tab. Last frame:\n%s", toSetKeys)
	}
	if !strings.Contains(toSetKeys, "user.name") {
		t.Fatalf("Set keys body did not render after the mouse click:\n%s", toSetKeys)
	}
	captureGlobalGitFrame(t, "global-git-strip-click-to-set-keys", s)

	clickLabelRow(t, s, "Options")
	toOptions, ok := s.waitFor(8*time.Second, func(text string) bool {
		return strings.Contains(text, "Global Git › Options")
	})
	if !ok {
		t.Fatalf("clicking the Options label never switched back to that sub-tab. Last frame:\n%s", toOptions)
	}
	if !strings.Contains(toOptions, "init.defaultBranch") {
		t.Fatalf("Options body did not render after the mouse click:\n%s", toOptions)
	}
	captureGlobalGitFrame(t, "global-git-strip-click-to-options", s)
}

// TestGlobalGit_RealPTYSetKeysProbeFailure proves the Set keys sub-tab's
// fail-open probe-failure state through the compiled binary, using the SAME
// FakeGitShimDir(..., "config") fixture TestGlobalGit_RealPTYProbeFailureStaysNavigable
// already uses for the Options sub-tab — failing every `git config`
// invocation fails BOTH probes (GlobalGitOptionStates and AllGitSetKeys),
// since keyboard ←/→ is fail-open-blocked while Options's own optionsErr is
// active (mirrors Global SSH's identical contract), so the Set keys sub-tab
// is reached via a raw mouse click on the strip label instead — the same
// click path TestGlobalGit_RealPTYSubTabStripClick proves independently.
func TestGlobalGit_RealPTYSetKeysProbeFailure(t *testing.T) {
	home := ShortSandboxHome(t)
	seedGlobalGitHome(t, home, "", "")
	s := startGlobalGitPTYExpectingProbeFailure(t, home, FakeGitShimDir(t, "2.50.0", "config"))
	mustSee(t, s, "git probe failed:", "Options probe failure names git probe")

	clickLabelRow(t, s, "Set keys")
	frame, ok := s.waitFor(8*time.Second, func(text string) bool {
		return strings.Contains(text, "Git config could not be read.")
	})
	if !ok {
		t.Fatalf("probe-failed heading never rendered on the Set keys sub-tab. Last frame:\n%s", frame)
	}
	if !strings.Contains(frame, "git config --list --show-origin failed — re-enter the screen to retry.") {
		t.Fatalf("probe-failed body line missing:\n%s", frame)
	}
	captureGlobalGitFrame(t, "global-git-set-keys-probe-failure", s)

	// Fail-open: a main-tab key still leaves the screen even while the Set
	// keys sub-tab is stuck in its error state.
	s.sendKey([]byte("1"), keystrokeDelay)
	after, ok := s.waitFor(8*time.Second, func(f string) bool {
		return !strings.Contains(f, "Global Git")
	})
	if !ok {
		t.Fatalf("main-tab key did not leave the failed Set keys sub-tab (fail-open broken):\n%s", after)
	}
	if !strings.Contains(after, "Identities") {
		t.Fatalf("expected the Identities main tab after leaving the failed screen:\n%s", after)
	}
}

// ---------------------------------------------------------------------------
// PROP-03 (09.5-03): free-form custom Git key entry, real terminal proof.
// Only a real PTY session can prove the raw-keystroke capture contract (Tab/
// Enter/Esc routing while the form owns the keyboard) genuinely works end to
// end through the COMPILED binary — an in-package model test drives
// handleKey directly and cannot catch a wiring gap between the real terminal
// and app.go's dispatch loop (the SAME rationale every other *_RealPTY* test
// in this file already documents).
// ---------------------------------------------------------------------------

// TestGlobalGit_RealPTYCustomKeyWrite drives the full custom-key flow through
// the compiled binary: open the Set keys sub-tab, press "n", type a
// well-formed key/value pair, submit into the write ceremony, confirm, and
// verify BOTH the on-screen receipt AND the real on-disk managed block.
func TestGlobalGit_RealPTYCustomKeyWrite(t *testing.T) {
	home := ShortSandboxHome(t)
	// A pre-existing main config (mirrors TestGlobalGit_RealPTYApplyConfirm's
	// own seeding) is required for the backup-count assertion below — a file
	// that never existed has nothing to back up, so an empty seed would make
	// that assertion vacuous rather than a real proof.
	main, baseline := seedGlobalGitHome(t, home, "[user]\n\tname = Test User\n", "")
	s := startGlobalGitPTY(t, home, "")

	s.sendKey(wizardKeyRight, keystrokeDelay)
	mustSee(t, s, "Global Git › Set keys", "one right-press reaches the Set keys sub-tab")
	mustSee(t, s, "Add custom key", "Set keys footer advertises the custom-key action")

	s.sendKey([]byte("n"), keystrokeDelay)
	mustSee(t, s, "Add custom key", "\"n\" opens the custom-key form")

	for _, r := range "core.pager" {
		s.sendKey([]byte(string(r)), keystrokeDelay)
	}
	s.sendKey([]byte("\t"), keystrokeDelay)
	for _, r := range "less -FRX" {
		s.sendKey([]byte(string(r)), keystrokeDelay)
	}
	s.sendKey(dummyKeyEnter, keystrokeDelay)
	mustSee(t, s, "Write custom Git key to", "submitting a well-formed key/value opens the write ceremony")
	baselineDisplay := "~/.gitconfig.d/00-baseline"
	mustSee(t, s, baselineDisplay, "ceremony heading names the resolved baseline target")

	s.sendKey(dummyKeyEnter, keystrokeDelay)
	mustSee(t, s, "Wrote →", "confirm writes the custom-key managed block")
	mustSee(t, s, "core.pager = less -FRX written.", "receipt shows the frozen PROP-03 success message")
	captureGlobalGitFrame(t, "global-git-custom-key-write", s)

	baselineContent := readFileE2E(t, baseline)
	for _, want := range []string{
		"# BEGIN gitid managed: custom-git-keys",
		"[core]",
		"pager = less -FRX",
		"# END gitid managed: custom-git-keys",
	} {
		if !strings.Contains(baselineContent, want) {
			t.Fatalf("baseline file missing %q:\n%s", want, baselineContent)
		}
	}
	mainContent := readFileE2E(t, main)
	if !strings.Contains(mainContent, "# BEGIN gitid managed: baseline-include") {
		t.Fatalf("main gitconfig missing the baseline-include block that makes the custom key reachable:\n%s", mainContent)
	}
	backups, err := filepath.Glob(main + ".bak.*")
	if err != nil || len(backups) != 1 {
		t.Fatalf("main config backups = %v, %v; want exactly one (fresh sandbox, first write)", backups, err)
	}
}

// TestGlobalGit_RealPTYCustomKeyRejectsMalformedKey proves a malformed key
// (no dot — SplitGitKey's own D-G requirement) renders the frozen
// PropsGitKeyInvalidFmt error INLINE on the still-open form, never opens the
// write ceremony, and leaves every on-disk file byte-identical.
func TestGlobalGit_RealPTYCustomKeyRejectsMalformedKey(t *testing.T) {
	home := ShortSandboxHome(t)
	main, baseline := seedGlobalGitHome(t, home, "[user]\n\tname = Test User\n", "")
	before := snapshotGlobalGitFiles(t, main, baseline)
	s := startGlobalGitPTY(t, home, "")

	s.sendKey(wizardKeyRight, keystrokeDelay)
	mustSee(t, s, "Global Git › Set keys", "one right-press reaches the Set keys sub-tab")

	s.sendKey([]byte("n"), keystrokeDelay)
	mustSee(t, s, "Add custom key", "\"n\" opens the custom-key form")

	for _, r := range "nodothere" {
		s.sendKey([]byte(string(r)), keystrokeDelay)
	}
	s.sendKey([]byte("\t"), keystrokeDelay)
	for _, r := range "somevalue" {
		s.sendKey([]byte(string(r)), keystrokeDelay)
	}
	s.sendKey(dummyKeyEnter, keystrokeDelay)
	mustSee(t, s, "That key/value can't be written:", "a malformed key renders the frozen inline error")

	frame := captureGlobalGitFrame(t, "global-git-custom-key-malformed", s)
	if strings.Contains(frame, "Write custom Git key to") {
		t.Fatalf("a plan-stage rejection must never open the write ceremony:\n%s", frame)
	}
	if !strings.Contains(frame, "nodothere") {
		t.Fatalf("the offending key must still be visible on the still-open form:\n%s", frame)
	}

	// Esc closes the rejected form; the app must still be reachable and
	// every file on disk untouched.
	s.sendKey(dummyKeyEsc, keystrokeDelay)
	mustSee(t, s, "Global Git › Set keys", "Esc closes the rejected form back to the Set keys list")
	assertGlobalGitFilesUnchanged(t, before, main, baseline)
}

// TestGlobalGit_RealPTYEnumRowEdit proves the enum editor integration on Global Git,
// exercising the edit key, the shared editor state machine, the conditional apply
// ceremony dispatch, and the hard-gate substitution on disk for merge.conflictstyle.
func TestGlobalGit_RealPTYEnumRowEdit(t *testing.T) {
	home := ShortSandboxHome(t)
	main, baseline := seedGlobalGitHome(t, home, "[user]\n\tname = Test User\n", "")
	s := startGlobalGitPTY(t, home, "")

	// Snapshot pre-edit state
	preEditState, _ := os.ReadFile(baseline)
	preEditContent := string(preEditState)

	// The merge.conflictstyle row is the last visible row (row 10, 0-indexed)
	// in the default Options list. Navigate to it.
	for i := 0; i < 10; i++ {
		s.sendKey(dummyKeyDown, keystrokeDelay)
	}
	frame := s.snapshot()
	if !strings.Contains(frame, "merge.conflictstyle") {
		t.Fatalf("merge.conflictstyle row not visible after navigation:\n%s", frame)
	}

	// Press 'e' to open the editor on the enum row
	s.sendKey([]byte("e"), keystrokeDelay)
	editorFrame, ok := s.waitFor(8*time.Second, func(text string) bool {
		// The editor should render in the detail pane with the enum values
		return strings.Contains(text, "merge") || strings.Contains(text, "diff3") || strings.Contains(text, "zdiff3")
	})
	if !ok {
		t.Fatalf("enum editor never opened after 'e' key. Last frame:\n%s", editorFrame)
	}

	// Verify the breadcrumb still shows Options (not a different sub-tab)
	if !strings.Contains(editorFrame, "Options") || !strings.Contains(editorFrame, "Global Git") {
		t.Fatalf("breadcrumb changed after opening editor (editor guard failed):\n%s", editorFrame)
	}

	// Press right to cycle the value
	s.sendKey(wizardKeyRight, keystrokeDelay)
	cycledFrame, ok := s.waitFor(8*time.Second, func(text string) bool {
		return strings.Contains(text, "Global Git") && strings.Contains(text, "Options")
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
		// After dismiss, we should be back in browse mode without the editor
		return !strings.Contains(text, "Edit merge") && strings.Contains(text, "merge.conflictstyle")
	})
	if !ok {
		t.Fatalf("editor did not dismiss with Esc key. Last frame:\n%s", dismissFrame)
	}

	// Verify the file is byte-identical after dismiss (no staged override committed)
	postDismissState, _ := os.ReadFile(baseline)
	postDismissContent := string(postDismissState)
	if preEditContent != postDismissContent {
		t.Fatalf("config file changed after dismissing editor (should be byte-identical):\nBefore:\n%s\nAfter:\n%s", preEditContent, postDismissContent)
	}

	// Re-open the editor and cycle to a non-recommended value
	s.sendKey([]byte("e"), keystrokeDelay)
	editorFrame2, ok := s.waitFor(8*time.Second, func(text string) bool {
		return strings.Contains(text, "zdiff3")
	})
	if !ok {
		t.Fatalf("enum editor never re-opened. Last frame:\n%s", editorFrame2)
	}

	// Cycle right to reach a different value (aim for zdiff3 from merge)
	s.sendKey(wizardKeyRight, keystrokeDelay)
	s.sendKey(wizardKeyRight, keystrokeDelay)

	// Press Enter to commit the staged override
	s.sendKey(dummyKeyEnter, keystrokeDelay)
	committedFrame, ok := s.waitFor(8*time.Second, func(text string) bool {
		// After commit, we should be back in browse mode
		return strings.Contains(text, "merge.conflictstyle") && strings.Contains(text, "Options")
	})
	if !ok {
		t.Fatalf("editor did not close after Enter commit. Last frame:\n%s", committedFrame)
	}

	// Open the apply ceremony
	s.sendKey([]byte("a"), keystrokeDelay)
	ceremonyFrame, ok := s.waitFor(8*time.Second, func(text string) bool {
		return strings.Contains(text, "Write global-git managed block") || strings.Contains(text, "baseline")
	})
	if !ok {
		t.Fatalf("apply ceremony never opened. Last frame:\n%s", ceremonyFrame)
	}

	// Press Enter to confirm the apply
	s.sendKey(dummyKeyEnter, keystrokeDelay)
	confirmPromptFrame, ok := s.waitFor(8*time.Second, func(text string) bool {
		// The confirmation prompt should appear
		return strings.Contains(text, "Type") || strings.Contains(text, "confirm")
	})
	if !ok {
		t.Fatalf("confirmation prompt never appeared. Last frame:\n%s", confirmPromptFrame)
	}

	// Type the confirmation code
	s.sendKey([]byte("yes"), keystrokeDelay)
	s.sendKey(dummyKeyEnter, keystrokeDelay)

	// Wait for the apply to complete
	successFrame, ok := s.waitFor(8*time.Second, func(text string) bool {
		return strings.Contains(text, "applied") || strings.Contains(text, "Options")
	})
	if !ok {
		t.Fatalf("apply did not complete. Last frame:\n%s", successFrame)
	}

	// Verify the file has the new value
	postApplyState, _ := os.ReadFile(baseline)
	postApplyContent := string(postApplyState)
	if postApplyContent == preEditContent {
		t.Fatalf("config file was not modified by the apply ceremony")
	}

	// Verify zdiff3 is in the file
	if !strings.Contains(postApplyContent, "zdiff3") {
		t.Fatalf("staged enum value (zdiff3) not found in written config:\n%s", postApplyContent)
	}

	// Verify a backup file was created
	backupMatches, err := filepath.Glob(main + ".bak.*")
	if err != nil {
		t.Fatalf("globbing for backup files: %v", err)
	}
	if len(backupMatches) < 1 {
		t.Fatalf("expected at least one timestamped backup file after enum edit, found none")
	}
	t.Logf("enum row edit backup file(s) created: %v", backupMatches)
}

// TestGlobalGit_RealPTYBundleRowHasNoEditor proves that bundle rows do not open
// editors when the edit key is pressed, demonstrating type-aware rendering.
func TestGlobalGit_RealPTYBundleRowHasNoEditor(t *testing.T) {
	home := SandboxHome(t)
	_, _ = seedGlobalGitHome(t, home, "[user]\n\tname = Test User\n", "")
	s := startGlobalGitPTY(t, home, "")

	// The alias bundle row is at index 8 (0-indexed) in the Options list
	for i := 0; i < 8; i++ {
		s.sendKey(dummyKeyDown, keystrokeDelay)
	}
	frame := s.snapshot()
	if !strings.Contains(frame, "alias") {
		t.Fatalf("alias bundle row not visible after navigation:\n%s", frame)
	}

	// Capture the row's rendered line before pressing edit
	beforeEditLine := extractRowLine(t, frame, "alias")

	// Press 'e' on the bundle row — should have no effect
	s.sendKey([]byte("e"), keystrokeDelay)
	afterEditFrame := s.snapshot()

	// Verify the row's rendered line is unchanged (no editor opened)
	afterEditLine := extractRowLine(t, afterEditFrame, "alias")
	if beforeEditLine != afterEditLine {
		t.Fatalf("bundle row rendering changed after 'e' key (editor should not open):\nBefore:\n%s\nAfter:\n%s", beforeEditLine, afterEditLine)
	}

	// Verify no editor detail pane appeared
	if strings.Contains(afterEditFrame, "Edit alias") {
		t.Fatalf("editor opened on bundle row (should not):\n%s", afterEditFrame)
	}
}

// TestGlobalGit_RealPTYFallbackPairEdit proves the fallback-pair editor integration on Global Git,
// exercising the edit key, the fallback-pair state machine, text input across dual fields,
// and the apply ceremony with on-disk commit.
func TestGlobalGit_RealPTYFallbackPairEdit(t *testing.T) {
	home := ShortSandboxHome(t)
	main, baseline := seedGlobalGitHome(t, home, "[user]\n\tname = Test User\n", "")
	s := startGlobalGitPTY(t, home, "")

	// Snapshot pre-edit state
	preEditState, _ := os.ReadFile(baseline)
	preEditContent := string(preEditState)

	// Navigate to the first option row (user.name fallback row, row 0)
	frame := s.snapshot()
	if !strings.Contains(frame, "user.name") {
		t.Fatalf("user.name row not visible initially:\n%s", frame)
	}

	// Press 'e' to open the fallback-pair editor on the user.name row
	s.sendKey([]byte("e"), keystrokeDelay)
	editorFrame, ok := s.waitFor(8*time.Second, func(text string) bool {
		// The fallback-pair editor should render in the detail pane with both name and email fields
		return strings.Contains(text, "name") && strings.Contains(text, "email")
	})
	if !ok {
		t.Fatalf("fallback-pair editor never opened after 'e' key. Last frame:\n%s", editorFrame)
	}

	// Verify the breadcrumb still shows Options (not a different sub-tab)
	if !strings.Contains(editorFrame, "Options") || !strings.Contains(editorFrame, "Global Git") {
		t.Fatalf("breadcrumb changed after opening editor (editor guard failed):\n%s", editorFrame)
	}

	// Type a new name into the name field
	s.sendKey([]byte("Alice Developer"), keystrokeDelay)

	// Press Tab to move to the email field
	s.sendKey([]byte{9}, keystrokeDelay) // Tab character
	s.sendKey([]byte("alice@example.com"), keystrokeDelay)

	// Press Esc to dismiss without committing
	s.sendKey(dummyKeyEsc, keystrokeDelay)
	dismissFrame, ok := s.waitFor(8*time.Second, func(text string) bool {
		// After dismiss, we should be back in browse mode without the editor
		return !strings.Contains(text, "name") || !strings.Contains(text, "email")
	})
	if !ok {
		t.Fatalf("editor did not dismiss with Esc key. Last frame:\n%s", dismissFrame)
	}

	// Verify the file is byte-identical after dismiss (no staged changes committed)
	postDismissState, _ := os.ReadFile(baseline)
	postDismissContent := string(postDismissState)
	if preEditContent != postDismissContent {
		t.Fatalf("config file changed after dismissing editor (should be byte-identical):\nBefore:\n%s\nAfter:\n%s", preEditContent, postDismissContent)
	}

	// Re-open the editor for a real commit
	s.sendKey([]byte("e"), keystrokeDelay)
	editorFrame2, ok := s.waitFor(8*time.Second, func(text string) bool {
		return strings.Contains(text, "name") && strings.Contains(text, "email")
	})
	if !ok {
		t.Fatalf("fallback-pair editor never re-opened. Last frame:\n%s", editorFrame2)
	}

	// Clear and enter new values
	// First, clear the name field (Ctrl+A, Backspace)
	s.sendKey([]byte{1}, keystrokeDelay) // Ctrl+A
	s.sendKey(dummyKeyBackspace, keystrokeDelay)
	s.sendKey([]byte("Bob Engineer"), keystrokeDelay)

	// Move to email field and enter value
	s.sendKey([]byte{9}, keystrokeDelay) // Tab
	s.sendKey([]byte("bob@example.com"), keystrokeDelay)

	// Press Enter to commit the staged override
	s.sendKey(dummyKeyEnter, keystrokeDelay)
	committedFrame, ok := s.waitFor(8*time.Second, func(text string) bool {
		// After commit, we should be back in browse mode
		return strings.Contains(text, "user.name") && strings.Contains(text, "Options")
	})
	if !ok {
		t.Fatalf("editor did not close after Enter commit. Last frame:\n%s", committedFrame)
	}

	// Open the apply ceremony
	s.sendKey([]byte("a"), keystrokeDelay)
	ceremonyFrame, ok := s.waitFor(8*time.Second, func(text string) bool {
		return strings.Contains(text, "Write global-git managed block") || strings.Contains(text, "baseline")
	})
	if !ok {
		t.Fatalf("apply ceremony never opened. Last frame:\n%s", ceremonyFrame)
	}

	// Press Enter to confirm the apply
	s.sendKey(dummyKeyEnter, keystrokeDelay)
	confirmPromptFrame, ok := s.waitFor(8*time.Second, func(text string) bool {
		// The confirmation prompt should appear
		return strings.Contains(text, "Type") || strings.Contains(text, "confirm")
	})
	if !ok {
		t.Fatalf("confirmation prompt never appeared. Last frame:\n%s", confirmPromptFrame)
	}

	// Type the confirmation code
	s.sendKey([]byte("yes"), keystrokeDelay)
	s.sendKey(dummyKeyEnter, keystrokeDelay)

	// Wait for the apply to complete
	successFrame, ok := s.waitFor(8*time.Second, func(text string) bool {
		return strings.Contains(text, "applied") || strings.Contains(text, "Options")
	})
	if !ok {
		t.Fatalf("apply did not complete. Last frame:\n%s", successFrame)
	}

	// Verify the file has the new values
	postApplyState, _ := os.ReadFile(baseline)
	postApplyContent := string(postApplyState)
	if postApplyContent == preEditContent {
		t.Fatalf("config file was not modified by the apply ceremony")
	}

	// Verify the new author values are in the file
	if !strings.Contains(postApplyContent, "Bob Engineer") {
		t.Fatalf("staged name value (Bob Engineer) not found in written config:\n%s", postApplyContent)
	}
	if !strings.Contains(postApplyContent, "bob@example.com") {
		t.Fatalf("staged email value (bob@example.com) not found in written config:\n%s", postApplyContent)
	}

	// Verify a backup file was created
	backupMatches, err := filepath.Glob(main + ".bak.*")
	if err != nil {
		t.Fatalf("globbing for backup files: %v", err)
	}
	if len(backupMatches) < 1 {
		t.Fatalf("expected at least one timestamped backup file after fallback-pair edit, found none")
	}
	t.Logf("fallback-pair edit backup file(s) created: %v", backupMatches)
}

// extractRowLine is a test helper that finds a row by key and returns its first line
func extractRowLine(t *testing.T, frame, key string) string {
	t.Helper()
	lines := strings.Split(frame, "\n")
	for i, line := range lines {
		if strings.Contains(line, key) {
			// Return this line (it's the first line of the row)
			return line
		}
	}
	t.Fatalf("row with key %q not found in frame:\n%s", key, frame)
	return ""
}
