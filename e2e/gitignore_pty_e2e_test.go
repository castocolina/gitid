//go:build e2e

package e2e

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/castocolina/gitid/internal/gitconfig"
)

const gitIgnoreBaselineMarker = "# BEGIN gitid managed: baseline"

var (
	gitIgnoreLineEnd        = []byte{0x05}
	gitIgnoreBracketedPaste = []byte("\x1b[200~gitignore-pty-paste-one\ngitignore-pty-paste-two\x1b[201~")
)

func seedGitIgnoreHome(t *testing.T, home string, foreignBefore, managedBody, foreignAfter, baselineShape string) (string, string) {
	t.Helper()
	gitdir := filepath.Join(home, ".gitconfig.d")
	if err := os.MkdirAll(gitdir, 0o700); err != nil {
		t.Fatalf("creating git config directory: %v", err)
	}

	gitignorePath := filepath.Join(home, ".gitignore_global")
	if foreignBefore != "" || managedBody != "" || foreignAfter != "" {
		content := foreignBefore
		if managedBody != "" {
			content += "# BEGIN gitid managed: gitignore\n" + managedBody + "\n# END gitid managed: gitignore\n"
		}
		content += foreignAfter
		writeFileT(t, gitignorePath, content)
	}

	baselinePath := filepath.Join(gitdir, "00-baseline")
	if baselineShape != "" {
		writeFileT(t, baselinePath, baselineShape)
	}

	return gitignorePath, baselinePath
}

func newGitIgnoreCmd(t *testing.T, ctx context.Context, bin, home string) *exec.Cmd {
	t.Helper()
	cmd := exec.CommandContext(ctx, bin)
	env, _ := e2eEnv(t, home)
	cmd.Env = append(env, "TERM=xterm-256color")
	return cmd
}

func startGitIgnorePTY(t *testing.T, home string) *ptySession {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second*ciTimeoutMultiplier())
	t.Cleanup(cancel)
	s := startPTYAt(t, newGitIgnoreCmd(t, ctx, BuildBinary(t), home), dummyTermWidth, dummyTermHeight)
	t.Cleanup(func() { s.close(t) })
	uiReady(t, s)
	s.sendKey([]byte("5"), keystrokeDelay)
	mustSee(t, s, "Global Git Ignore", "GitIgnore screen opens")
	return s
}

func captureGitIgnoreFrame(t *testing.T, name string, s *ptySession) string {
	t.Helper()
	frame := s.snapshot()
	saveFrame(t, name, s)
	return frame
}

func snapshotGitIgnoreFiles(t *testing.T, paths ...string) map[string][]byte {
	t.Helper()
	return snapshotGitBytes(t, paths)
}

func assertGitIgnoreFilesUnchanged(t *testing.T, before map[string][]byte, paths ...string) {
	t.Helper()
	assertGitBytesUnchanged(t, before, snapshotGitIgnoreFiles(t, paths...))
}

func TestGitIgnore_RealPTYSeedsFromDefaultsWhenAbsent(t *testing.T) {
	home := SandboxHome(t)
	seedGitIgnoreHome(t, home, "", "", "",
		"# BEGIN gitid managed: baseline\n[core]\n\tignorecase = false\n# END gitid managed: baseline\n")
	s := startGitIgnorePTY(t, home)
	frame := captureGitIgnoreFrame(t, "gitignore-seeds-from-defaults", s)
	for _, want := range []string{".DS_Store", "No managed block found yet"} {
		if !strings.Contains(frame, want) {
			t.Fatalf("defaults frame missing %q:\n%s", want, frame)
		}
	}
}

func seedEditableGitIgnoreHome(t *testing.T, home string, body string) (string, string) {
	t.Helper()
	return seedGitIgnoreHome(t, home, "# foreign before\n", body, "# foreign after\n", "# BEGIN gitid managed: baseline\n[core]\n\texcludesfile = ~/.gitignore_global\n# END gitid managed: baseline\n")
}

func appendGitIgnoreLine(t *testing.T, s *ptySession, line string, seededLines int) {
	t.Helper()
	s.sendKey(dummyKeyEnter, keystrokeDelay)
	for range seededLines {
		s.sendKey(dummyKeyDown, keystrokeDelay)
	}
	s.sendKey(gitIgnoreLineEnd, keystrokeDelay)
	s.sendKey(dummyKeyEnter, keystrokeDelay)
	s.sendKey([]byte(line), keystrokeDelay)
	s.sendKey(dummyKeyEsc, keystrokeDelay)
}

func applyGitIgnoreAndConfirm(t *testing.T, s *ptySession) {
	t.Helper()
	s.sendKey([]byte("a"), keystrokeDelay)
	mustSee(t, s, "Review your global gitignore", "edited content opens review")
	s.sendKey(dummyKeyEnter, keystrokeDelay)
	mustSee(t, s, "Wrote", "review confirmation writes edited content")
}

func TestGitIgnore_RealPTYShowsExistingManagedBlock(t *testing.T) {
	home := SandboxHome(t)
	managedContent := ".DS_Store\n*.log"
	foreignBefore := "# user-added\n"
	foreignAfter := "\n# also user-added\n"
	baselineWithWiring := "# BEGIN gitid managed: baseline\n[core]\n\tignorecase = false\n\texcludesfile = ~/.gitignore_global\n# END gitid managed: baseline\n"

	seedGitIgnoreHome(t, home, foreignBefore, managedContent, foreignAfter, baselineWithWiring)
	s := startGitIgnorePTY(t, home)
	frame := captureGitIgnoreFrame(t, "gitignore-shows-existing-managed", s)

	for _, want := range []string{".DS_Store", "*.log"} {
		if !strings.Contains(frame, want) {
			t.Fatalf("frame missing managed content %q:\n%s", want, frame)
		}
	}
	if strings.Contains(frame, "user-added") {
		t.Fatalf("frame must not show foreign lines:\n%s", frame)
	}
	if !strings.Contains(frame, "Wired") || !strings.Contains(frame, "~/.gitignore_global") {
		t.Fatalf("frame must show wiring sentence about managed target:\n%s", frame)
	}
	if strings.Contains(frame, "points at") && strings.Contains(frame, "instead of this file") {
		t.Fatalf("frame must NOT show wrong-target sentence:\n%s", frame)
	}
}

func TestGitIgnore_RealPTYCancelWritesNothing(t *testing.T) {
	home := SandboxHome(t)
	gitignorePath, _ := seedGitIgnoreHome(t, home, "", ".DS_Store\n*.log", "",
		"# BEGIN gitid managed: baseline\n[core]\n\tignorecase = false\n# END gitid managed: baseline\n")
	before := snapshotGitIgnoreFiles(t, gitignorePath)
	s := startGitIgnorePTY(t, home)
	s.sendKey([]byte("a"), keystrokeDelay)
	mustSee(t, s, "Review your global gitignore", "apply opens ceremony")
	captureGitIgnoreFrame(t, "gign-review-ceremony", s)
	s.sendKey(dummyKeyEsc, keystrokeDelay)
	mustSee(t, s, "Global Git Ignore", "cancel returns to screen")
	captureGitIgnoreFrame(t, "gitignore-cancel-writes-nothing", s)
	assertGitIgnoreFilesUnchanged(t, before, gitignorePath)

	backups, _ := filepath.Glob(gitignorePath + ".bak.*")
	if len(backups) > 0 {
		t.Fatalf("cancel must not create backups: %v", backups)
	}
}

func TestGitIgnore_RealPTYConfirmWritesBlockAndBacksUp(t *testing.T) {
	home := SandboxHome(t)
	foreignBefore := "# user added before\n"
	foreignAfter := "\n# user added after\n"
	gitignorePath, _ := seedGitIgnoreHome(t, home, foreignBefore, "", foreignAfter,
		"# BEGIN gitid managed: baseline\n[core]\n\tignorecase = false\n# END gitid managed: baseline\n")

	beforeContent, err := os.ReadFile(gitignorePath)
	if err != nil {
		t.Fatalf("reading gitignore before test: %v", err)
	}

	s := startGitIgnorePTY(t, home)
	s.sendKey([]byte("a"), keystrokeDelay)
	mustSee(t, s, "Review your global gitignore", "apply opens ceremony")
	s.sendKey(dummyKeyEnter, keystrokeDelay)
	mustSee(t, s, "Wrote", "confirm writes block")
	frame := captureGitIgnoreFrame(t, "gitignore-confirm-writes-block", s)

	if !strings.Contains(frame, "Backed up") {
		t.Fatalf("receipt must show backup:\n%s", frame)
	}

	backups, err := filepath.Glob(gitignorePath + ".bak.*")
	if err != nil || len(backups) != 1 {
		t.Fatalf("confirm must create exactly one backup: %v, %v", backups, err)
	}

	afterContent, err := os.ReadFile(gitignorePath)
	if err != nil {
		t.Fatalf("reading gitignore after test: %v", err)
	}

	if !strings.Contains(string(afterContent), "user added before") || !strings.Contains(string(afterContent), "user added after") {
		t.Fatalf("foreign content must be preserved: %s", afterContent)
	}

	if !strings.Contains(string(afterContent), "BEGIN gitid managed: gitignore") {
		t.Fatalf("managed block must be present: %s", afterContent)
	}

	if bytes.Equal(beforeContent, afterContent) {
		t.Fatalf("content must have changed after confirm")
	}
}

func TestGitIgnore_RealPTYSecondConfirmIsIdempotent(t *testing.T) {
	home := SandboxHome(t)
	gitignorePath, baselinePath := seedGitIgnoreHome(t, home, "# existing\n", "", "",
		"# BEGIN gitid managed: baseline\n[core]\n\tignorecase = false\n# END gitid managed: baseline\n")

	s := startGitIgnorePTY(t, home)
	s.sendKey([]byte("a"), keystrokeDelay)
	mustSee(t, s, "Review your global gitignore", "first apply opens ceremony")
	s.sendKey(dummyKeyEnter, keystrokeDelay)
	mustSee(t, s, "Wrote", "first confirm writes")
	s.sendKey(dummyKeyEnter, keystrokeDelay)
	mustSee(t, s, "Global Git Ignore", "first receipt dismissed")

	afterFirst, err := os.ReadFile(gitignorePath)
	if err != nil {
		t.Fatalf("reading gitignore after first write: %v", err)
	}

	s.sendKey([]byte("a"), keystrokeDelay)
	mustSee(t, s, "Review your global gitignore", "second apply opens ceremony")
	s.sendKey(dummyKeyEnter, keystrokeDelay)
	mustSee(t, s, "Wrote", "second confirm writes")
	frame := captureGitIgnoreFrame(t, "gitignore-idempotent-confirm", s)

	if strings.Contains(frame, "Backed up") {
		t.Fatalf("idempotent second confirm must show no-backup needed:\n%s", frame)
	}

	afterSecond, err := os.ReadFile(gitignorePath)
	if err != nil {
		t.Fatalf("reading gitignore after second write: %v", err)
	}

	if !bytes.Equal(afterFirst, afterSecond) {
		t.Fatalf("idempotent second write must not change content\nFirst write:\n%s\nSecond write:\n%s", afterFirst, afterSecond)
	}

	gitignoreBackups, _ := filepath.Glob(gitignorePath + ".bak.*")
	if len(gitignoreBackups) != 1 {
		t.Fatalf("gitignore should have exactly one backup from first confirm (content changed): %v", gitignoreBackups)
	}

	baselineBackups, _ := filepath.Glob(baselinePath + ".bak.*")
	if len(baselineBackups) != 1 {
		t.Fatalf("baseline should have exactly one backup from first confirm (excludesfile key added): %v", baselineBackups)
	}
}

func TestGitIgnore_RealPTYWiresExcludesfileWhenUnset(t *testing.T) {
	home := SandboxHome(t)
	_, baselinePath := seedGitIgnoreHome(t, home, "", "", "",
		"# BEGIN gitid managed: baseline\n[core]\n\tignorecase = false\n# END gitid managed: baseline\n")

	s := startGitIgnorePTY(t, home)
	mustSee(t, s, "also set core.excludesfile in your Git baseline", "two-target note appears in main view")
	s.sendKey([]byte("a"), keystrokeDelay)
	mustSee(t, s, "Review your global gitignore", "apply opens ceremony")
	s.sendKey(dummyKeyEnter, keystrokeDelay)
	mustSee(t, s, "Wrote", "confirm writes")
	frame := captureGitIgnoreFrame(t, "gitignore-wires-excludesfile", s)

	if !strings.Contains(frame, "baseline") {
		t.Fatalf("receipt must mention baseline:\n%s", frame)
	}

	baselineContent, err := os.ReadFile(baselinePath)
	if err != nil {
		t.Fatalf("reading baseline after test: %v", err)
	}

	if !strings.Contains(string(baselineContent), "excludesfile = ~/.gitignore_global") {
		t.Fatalf("baseline must have excludesfile line:\n%s", baselineContent)
	}

	if !strings.Contains(string(baselineContent), "ignorecase = false") {
		t.Fatalf("baseline other lines must survive:\n%s", baselineContent)
	}
}

func TestGitIgnore_RealPTYRefusesWhenNoBaselineBlock(t *testing.T) {
	home := SandboxHome(t)
	seedGitIgnoreHome(t, home, "", "", "", "")

	s := startGitIgnorePTY(t, home)
	frame := captureGitIgnoreFrame(t, "gign-error-no-baseline-block", s)

	if !strings.Contains(frame, "No gitid-managed Git baseline") {
		t.Fatalf("frame must show no-baseline error:\n%s", frame)
	}

	s.sendKey([]byte("a"), keystrokeDelay)
	before := s.snapshot()
	time.Sleep(100 * time.Millisecond)
	after := s.snapshot()
	if before != after {
		t.Fatalf("apply must be inert when no baseline block exists")
	}

	gitignorePath := filepath.Join(home, ".gitignore_global")
	_, err := os.Stat(gitignorePath)
	if err == nil {
		t.Fatalf("no file should be created when baseline is absent")
	}
}

func TestGitIgnore_RealPTYRefusesMalformedFile(t *testing.T) {
	home := SandboxHome(t)
	baselineBlock := "# BEGIN gitid managed: baseline\n[core]\n\tignorecase = false\n# END gitid managed: baseline\n"

	gitignorePath, _ := seedGitIgnoreHome(t, home, "", "# BEGIN gitid managed: gitignore\n.DS_Store\n", "",
		baselineBlock)

	s := startGitIgnorePTY(t, home)
	frame := captureGitIgnoreFrame(t, "gign-error-malformed-file", s)

	if !strings.Contains(frame, "repair the file by hand") {
		t.Fatalf("frame must show malformed-file error:\n%s", frame)
	}

	s.sendKey([]byte("a"), keystrokeDelay)
	before := s.snapshot()
	time.Sleep(100 * time.Millisecond)
	after := s.snapshot()
	if before != after {
		t.Fatalf("apply must be inert when file is malformed")
	}

	beforeBytes := snapshotGitIgnoreFiles(t, gitignorePath)
	time.Sleep(500 * time.Millisecond)
	assertGitIgnoreFilesUnchanged(t, beforeBytes, gitignorePath)

	t.Run("dual-blocks", func(t *testing.T) {
		home := SandboxHome(t)
		gitignorePath, _ := seedGitIgnoreHome(t, home, "", "", "",
			"# BEGIN gitid managed: baseline\n[core]\n\tignorecase = false\n# END gitid managed: baseline\n"+
				"# BEGIN gitid managed: baseline\n[core]\n\texcludesfile = ~/.gitignore_global\n# END gitid managed: baseline\n")

		s := startGitIgnorePTY(t, home)
		frame := captureGitIgnoreFrame(t, "gign-error-malformed-file-dual", s)

		if !strings.Contains(frame, "repair the file by hand") {
			t.Fatalf("frame must show malformed-file error for duplicate blocks:\n%s", frame)
		}

		beforeBytes := snapshotGitIgnoreFiles(t, gitignorePath)
		s.sendKey([]byte("a"), keystrokeDelay)
		time.Sleep(100 * time.Millisecond)
		assertGitIgnoreFilesUnchanged(t, beforeBytes, gitignorePath)
	})
}

func TestGitIgnore_RealPTYRefusesDuplicateBaselineBlock(t *testing.T) {
	home := SandboxHome(t)
	gitconfigDir := filepath.Join(home, ".gitconfig.d")
	if err := os.MkdirAll(gitconfigDir, 0o700); err != nil {
		t.Fatalf("creating git config dir: %v", err)
	}

	baselinePath := filepath.Join(gitconfigDir, "00-baseline")
	baselineContent := "# BEGIN gitid managed: baseline\n[core]\n\tignorecase = false\n# END gitid managed: baseline\n" +
		"# BEGIN gitid managed: baseline\n[core]\n\texcludesfile = ~/.gitignore_global\n# END gitid managed: baseline\n"
	writeFileT(t, baselinePath, baselineContent)

	s := startGitIgnorePTY(t, home)
	frame := captureGitIgnoreFrame(t, "gign-error-duplicate-baseline-block", s)

	if !strings.Contains(frame, "two complete") || !strings.Contains(frame, "repair the file by hand") {
		t.Fatalf("frame must show malformed-file error for duplicate blocks:\n%s", frame)
	}

	s.sendKey([]byte("a"), keystrokeDelay)
	before := s.snapshot()
	time.Sleep(100 * time.Millisecond)
	after := s.snapshot()
	if before != after {
		t.Fatalf("apply must be inert when duplicate blocks exist")
	}

	beforeBytes := snapshotGitIgnoreFiles(t, baselinePath)
	time.Sleep(500 * time.Millisecond)
	assertGitIgnoreFilesUnchanged(t, beforeBytes, baselinePath)
}

func TestGitIgnore_RealPTYEditThenWritePersistsUserLine(t *testing.T) {
	home := SandboxHome(t)
	gitignorePath, _ := seedEditableGitIgnoreHome(t, home, ".DS_Store\n*.log")
	s := startGitIgnorePTY(t, home)
	appendGitIgnoreLine(t, s, "gitignore-user-line", 2)
	applyGitIgnoreAndConfirm(t, s)
	frame := captureGitIgnoreFrame(t, "gitignore-edit-write", s)
	if !strings.Contains(frame, "Wrote") {
		t.Fatalf("write receipt missing:\n%s", frame)
	}
	content := readFileE2E(t, gitignorePath)
	for _, want := range []string{"# foreign before\n", "gitignore-user-line", "# foreign after\n"} {
		if !strings.Contains(content, want) {
			t.Fatalf("written gitignore missing %q:\n%s", want, content)
		}
	}
	backups, err := filepath.Glob(gitignorePath + ".bak.*")
	if err != nil || len(backups) != 1 {
		t.Fatalf("expected one timestamped backup, got %v (%v)", backups, err)
	}
}

func TestGitIgnore_RealPTYEditThenResetDiscardsEdit(t *testing.T) {
	home := SandboxHome(t)
	seedEditableGitIgnoreHome(t, home, ".DS_Store\n*.log")
	s := startGitIgnorePTY(t, home)
	appendGitIgnoreLine(t, s, "gitignore-reset-marker", 2)
	s.sendKey([]byte("r"), keystrokeDelay)
	frame := captureGitIgnoreFrame(t, "gitignore-edit-reset", s)
	if strings.Contains(frame, "gitignore-reset-marker") || !strings.Contains(frame, ".DS_Store") {
		t.Fatalf("reset did not restore curated content:\n%s", frame)
	}
}

func TestGitIgnore_RealPTYResetAloneWritesNothing(t *testing.T) {
	home := SandboxHome(t)
	gitignorePath, _ := seedEditableGitIgnoreHome(t, home, ".DS_Store\n*.log")
	before := snapshotGitIgnoreFiles(t, gitignorePath)
	s := startGitIgnorePTY(t, home)
	s.sendKey([]byte("r"), keystrokeDelay)
	captureGitIgnoreFrame(t, "gitignore-reset-no-write", s)
	assertGitIgnoreFilesUnchanged(t, before, gitignorePath)
	backups, _ := filepath.Glob(gitignorePath + ".bak.*")
	if len(backups) != 0 {
		t.Fatalf("reset must not create backups: %v", backups)
	}
}

func TestGitIgnore_RealPTYDeletedDefaultStaysDeleted(t *testing.T) {
	home := SandboxHome(t)
	lines := []string{".DS_Store", "*.log", ".env", "node_modules/"}
	deleteIndex := -1
	for i, line := range lines {
		if line == ".env" {
			deleteIndex = i
		}
	}
	if deleteIndex != 2 {
		t.Fatalf("fixture .env index = %d, want 2", deleteIndex)
	}
	curated := false
	for _, entry := range gitconfig.DefaultGitignoreEntries() {
		if entry == ".env" {
			curated = true
		}
	}
	if !curated {
		t.Fatal(".env must remain a curated default entry")
	}
	gitignorePath, _ := seedEditableGitIgnoreHome(t, home, strings.Join(lines, "\n"))
	s := startGitIgnorePTY(t, home)
	s.sendKey(dummyKeyEnter, keystrokeDelay)
	for i := 0; i < deleteIndex; i++ {
		s.sendKey(dummyKeyDown, keystrokeDelay)
	}
	s.sendKey([]byte("\x1b[H"), keystrokeDelay)
	s.sendKey([]byte{0x0b}, keystrokeDelay)
	s.sendKey([]byte{0x7f}, keystrokeDelay)
	s.sendKey(dummyKeyEsc, keystrokeDelay)
	applyGitIgnoreAndConfirm(t, s)
	s.sendKey(dummyKeyEnter, keystrokeDelay)
	s.sendKey([]byte("1"), keystrokeDelay)
	s.sendKey([]byte("5"), keystrokeDelay)
	mustSee(t, s, "Global Git Ignore", "re-enter ignore screen")
	frame := captureGitIgnoreFrame(t, "gitignore-deleted-default", s)
	if strings.Contains(frame, ".env") || strings.Contains(readFileE2E(t, gitignorePath), "\n.env\n") {
		t.Fatalf("deleted .env survived:\n%s", frame)
	}
}

func TestGitIgnore_RealPTYShortcutLettersAreTypedNotTriggered(t *testing.T) {
	home := SandboxHome(t)
	seedEditableGitIgnoreHome(t, home, ".DS_Store\n*.log")
	s := startGitIgnorePTY(t, home)
	s.sendKey(dummyKeyEnter, keystrokeDelay)
	s.sendKey([]byte("rae"), keystrokeDelay)
	frame := captureGitIgnoreFrame(t, "gitignore-shortcut-letters", s)
	if !strings.Contains(frame, "rae") || strings.Contains(frame, "Review your global gitignore") {
		t.Fatalf("shortcut letters were not typed in editor:\n%s", frame)
	}
	s.sendKey(dummyKeyEsc, keystrokeDelay)
}

func TestGitIgnore_RealPTYTypedSentinelIsRefused(t *testing.T) {
	home := SandboxHome(t)
	gitignorePath, baselinePath := seedEditableGitIgnoreHome(t, home, ".DS_Store\n*.log")
	before := snapshotGitIgnoreFiles(t, gitignorePath, baselinePath)
	s := startGitIgnorePTY(t, home)
	appendGitIgnoreLine(t, s, "# BEGIN gitid managed: typed-sentinel", 2)
	s.sendKey([]byte("a"), keystrokeDelay)
	frame := captureGitIgnoreFrame(t, "gign-error-sentinel-rejected", s)
	if !strings.Contains(frame, "sentinel") || strings.Contains(frame, "Review your global gitignore") {
		t.Fatalf("typed sentinel was not refused:\n%s", frame)
	}
	assertGitIgnoreFilesUnchanged(t, before, gitignorePath, baselinePath)
}

func TestGitIgnore_RealPTYBracketedPasteReachesEditor(t *testing.T) {
	home := SandboxHome(t)
	gitignorePath, _ := seedEditableGitIgnoreHome(t, home, ".DS_Store\n*.log")
	s := startGitIgnorePTY(t, home)
	s.sendKey(dummyKeyEnter, keystrokeDelay)
	s.sendKey(gitIgnoreBracketedPaste, keystrokeDelay)
	frame := captureGitIgnoreFrame(t, "gitignore-bracketed-paste", s)
	for _, want := range []string{"gitignore-pty-paste-one", "gitignore-pty-paste-two"} {
		if !strings.Contains(frame, want) {
			t.Fatalf("bracketed paste missing %q:\n%s", want, frame)
		}
	}
	s.sendKey(dummyKeyEsc, keystrokeDelay)
	applyGitIgnoreAndConfirm(t, s)
	content := readFileE2E(t, gitignorePath)
	if !strings.Contains(content, "gitignore-pty-paste-one") || !strings.Contains(content, "gitignore-pty-paste-two") {
		t.Fatalf("bracketed paste was not persisted:\n%s", content)
	}
}

func TestGitIgnore_RealPTYChangedSincePreview(t *testing.T) {
	home := SandboxHome(t)
	managedContent := ".DS_Store\n*.log"
	gitignorePath, baselinePath := seedGitIgnoreHome(t, home, "", managedContent, "",
		"# BEGIN gitid managed: baseline\n[core]\n\tignorecase = false\n\texcludesfile = ~/.gitignore_global\n# END gitid managed: baseline\n")

	beforeGitignore, err := os.ReadFile(gitignorePath)
	if err != nil {
		t.Fatalf("reading gitignore before test: %v", err)
	}
	_ = baselinePath
	_, err = os.ReadFile(baselinePath)
	if err != nil {
		t.Fatalf("reading baseline before test: %v", err)
	}

	s := startGitIgnorePTY(t, home)
	s.sendKey([]byte("a"), keystrokeDelay)
	_, ok := s.waitFor(15*time.Second, func(text string) bool {
		return strings.Contains(text, "Review your global gitignore")
	})
	if !ok {
		t.Fatalf("ceremony never opened:\n%s", s.snapshot())
	}

	externalGitignoreEdit := string(beforeGitignore) + "\n# EXTERNAL EDIT — added after preview\n"
	if err := os.WriteFile(gitignorePath, []byte(externalGitignoreEdit), 0o600); err != nil {
		t.Fatalf("writing external edit to gitignore: %v", err)
	}

	s.sendKey(dummyKeyEnter, keystrokeDelay)

	_, ok = s.waitFor(30*time.Second, func(text string) bool {
		return strings.Contains(text, "changed since")
	})
	if !ok {
		t.Fatalf("changed-since-preview refusal never rendered:\n%s", s.snapshot())
	}

	refusalFrame := captureGitIgnoreFrame(t, "gign-error-changed-since-preview", s)

	if !strings.Contains(refusalFrame, "changed since you last reviewed") {
		t.Fatalf("refusal frame must show the frozen error copy:\n%s", refusalFrame)
	}

	if strings.Contains(refusalFrame, "Backed up") || strings.Contains(refusalFrame, "Wrote") {
		t.Fatalf("no receipt should appear on changed-since-preview refusal:\n%s", refusalFrame)
	}

	backupGitignore, _ := filepath.Glob(gitignorePath + ".bak.*")
	if len(backupGitignore) > 0 {
		t.Fatalf("no backup should be created for gitignore on refusal: %v", backupGitignore)
	}

	backupBaseline, _ := filepath.Glob(baselinePath + ".bak.*")
	if len(backupBaseline) > 0 {
		t.Fatalf("no backup should be created for baseline on refusal: %v", backupBaseline)
	}

	afterGitignore, err := os.ReadFile(gitignorePath)
	if err != nil {
		t.Fatalf("reading gitignore after test: %v", err)
	}
	if !bytes.Equal([]byte(externalGitignoreEdit), afterGitignore) {
		t.Fatalf("external edit must survive unchanged")
	}

	t.Run("baseline-mutation", func(t *testing.T) {
		home := SandboxHome(t)
		managedContent := ".DS_Store\n*.log"
		gitignorePath, baselinePath := seedGitIgnoreHome(t, home, "", managedContent, "",
			"# BEGIN gitid managed: baseline\n[core]\n\tignorecase = false\n\texcludesfile = ~/.gitignore_global\n# END gitid managed: baseline\n")

		beforeGitignore, _ := os.ReadFile(gitignorePath)
		beforeBaseline, _ := os.ReadFile(baselinePath)

		s := startGitIgnorePTY(t, home)
		s.sendKey([]byte("a"), keystrokeDelay)
		_, ok := s.waitFor(15*time.Second, func(text string) bool {
			return strings.Contains(text, "Review your global gitignore")
		})
		if !ok {
			t.Fatalf("ceremony never opened")
		}

		externalBaselineEdit := string(beforeBaseline) + "\n# EXTERNAL EDIT TO BASELINE\n"
		if err := os.WriteFile(baselinePath, []byte(externalBaselineEdit), 0o600); err != nil {
			t.Fatalf("writing external edit to baseline: %v", err)
		}

		s.sendKey(dummyKeyEnter, keystrokeDelay)

		_, ok = s.waitFor(30*time.Second, func(text string) bool {
			return strings.Contains(text, "changed since")
		})
		if !ok {
			t.Fatalf("changed-since-preview refusal never rendered on baseline mutation")
		}

		refusalFrame := captureGitIgnoreFrame(t, "gign-error-changed-since-preview-baseline", s)
		if !strings.Contains(refusalFrame, "changed since") {
			t.Fatalf("refusal must appear on baseline mutation:\n%s", refusalFrame)
		}

		afterGitignore, _ := os.ReadFile(gitignorePath)
		afterBaseline, _ := os.ReadFile(baselinePath)

		if !bytes.Equal(beforeGitignore, afterGitignore) {
			t.Fatalf("gitignore must not be modified on baseline mutation refusal")
		}
		if !bytes.Equal([]byte(externalBaselineEdit), afterBaseline) {
			t.Fatalf("baseline external edit must survive unchanged on refusal")
		}
	})
}
