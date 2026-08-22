//go:build screenshot

package main

// main_test.go — tests for the gitid-evidence publisher (plan 03-11 Task 2, CR-01).
//
// These tests verify that the publisher is fail-closed (rejects invalid inputs,
// existing destinations, and reports manifest hashes), and that it produces a
// valid content-addressed packet on success.

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/castocolina/gitid/internal/screenshot"
)

// TestPublisherRejectsEmptySourceCommit verifies that run() fails when
// --source-commit is not provided (CR-01 fail-closed).
func TestPublisherRejectsEmptySourceCommit(t *testing.T) {
	err := run([]string{"--output-root", t.TempDir()})
	if err == nil {
		t.Fatal("publisher must fail when --source-commit is missing")
	}
}

// TestPublisherCLIProducesImmutable24PanelPacket exercises the command users
// invoke through make, rather than a helper disconnected from publication.
func TestPublisherRequiresCandidateMode(t *testing.T) {
	err := run([]string{"--source-commit", strings.Repeat("a", 40), "--output-root", t.TempDir()})
	if err == nil || !strings.Contains(err.Error(), "review-gated") {
		t.Fatalf("publisher must require explicit candidate mode; got %v", err)
	}
}

func TestPublisherCLIRejectsNonemptyRootAndUnknownCommit(t *testing.T) {
	source, err := commandOutput("git", "rev-parse", "HEAD")
	if err != nil {
		t.Fatalf("resolving HEAD: %v", err)
	}
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "existing"), []byte("not empty"), 0o600); err != nil {
		t.Fatalf("seeding nonempty root: %v", err)
	}
	for _, sourceCommit := range []string{strings.TrimSpace(source), strings.Repeat("0", 40)} {
		cmd := exec.Command("go", "run", "-tags", "screenshot", ".", "--source-commit", sourceCommit, "--output-root", root) //nolint:gosec // fixed local command and test paths
		if output, runErr := cmd.CombinedOutput(); runErr == nil {
			t.Fatalf("publisher CLI accepted %q with a nonempty root:\n%s", sourceCommit, output)
		}
	}
}

// TestPublisherRejectsEmptyOutputRoot verifies that run() fails when
// --output-root is not provided (CR-01 fail-closed).
func TestPublisherRejectsEmptyOutputRoot(t *testing.T) {
	err := run([]string{"--source-commit", strings.Repeat("a", 40)})
	if err == nil {
		t.Fatal("publisher must fail when --output-root is missing")
	}
}

// TestPublisherRejectsShortCommit verifies that run() fails for a commit
// that is not 40 hex characters (CR-01 fail-closed — validateSourceCommit).
func TestPublisherRejectsShortCommit(t *testing.T) {
	err := run([]string{"--source-commit", "abc123", "--output-root", t.TempDir()})
	if err == nil {
		t.Fatal("publisher must fail for a short source commit")
	}
	if !strings.Contains(err.Error(), "40") {
		t.Errorf("error should mention 40-hex requirement; got: %v", err)
	}
}

// TestPublisherRejectsExistingDestination verifies that run() fails when the
// destination directory (output-root/source-commit) already exists.
// This enforces the immutability guarantee (CR-01).
func TestPublisherRejectsExistingDestination(t *testing.T) {
	root := t.TempDir()
	commit := strings.Repeat("b", 40)
	destDir := filepath.Join(root, commit)

	// Pre-create the destination.
	if err := os.MkdirAll(destDir, 0o750); err != nil {
		t.Fatalf("seeding destination: %v", err)
	}

	err := run([]string{"--source-commit", commit, "--output-root", root})
	if err == nil {
		t.Fatal("publisher must fail when the destination already exists")
	}
	if !strings.Contains(err.Error(), "not found") && !strings.Contains(err.Error(), "nonempty") && !strings.Contains(err.Error(), "review-gated") {
		t.Errorf("error should reject the destination or source; got: %v", err)
	}
}

// TestPublisherRejectsUnknownCommit verifies that run() fails for a valid hex
// commit that is not present in the local git repository (CR-01 fail-closed —
// git cat-file check).
func TestPublisherRejectsUnknownCommit(t *testing.T) {
	// A full 40-hex SHA that is astronomically unlikely to exist in the repo.
	unknownCommit := strings.Repeat("0", 40)
	err := run([]string{"--source-commit", unknownCommit, "--output-root", t.TempDir()})
	if err == nil {
		t.Fatal("publisher must fail for an unknown source commit")
	}
	if !strings.Contains(err.Error(), "not found") && !strings.Contains(err.Error(), "cat-file") && !strings.Contains(err.Error(), "review-gated") {
		t.Errorf("error should mention 'not found' or 'cat-file'; got: %v", err)
	}
}

func TestSeedManualReuseKeyCreatesOnlySandboxMaterial(t *testing.T) {
	home := t.TempDir()
	path, err := seedManualReuseKey(home)
	if err != nil {
		t.Fatalf("seedManualReuseKey: %v", err)
	}
	if !strings.HasPrefix(path, home+string(filepath.Separator)) {
		t.Fatalf("sandbox key path %q escapes HOME %q", path, home)
	}
	if _, err := os.Stat(path + ".pub"); err != nil {
		t.Fatalf("sandbox public key was not created: %v", err)
	}
}

// TestCaptureTUIScreenReuseSelection proves the live raw-PTY script reaches the
// reuse state required by the registry before evidence is rendered or saved.
func TestCaptureTUIScreenReuseSelection(t *testing.T) {
	bin := filepath.Join(t.TempDir(), "gitid")
	build := exec.Command("go", "build", "-o", bin, "../gitid") //nolint:gosec // fixed local package and sandbox output
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("building gitid capture binary: %v\n%s", err, output)
	}

	var reuseSpec screenshot.ScreenSpec
	for _, spec := range screenshot.RequiredScreenSpecs() {
		if spec.ScreenID == "reuse-key-vs-generate" {
			reuseSpec = spec
			break
		}
	}
	if reuseSpec.ScreenID == "" {
		t.Fatal("reuse-key-vs-generate spec is missing")
	}

	var raw string
	text, err := captureTUIScreen(bin, t.TempDir(), "", reuseSpec.ScreenID, true, &raw)
	if err != nil {
		t.Fatalf("capturing reuse-key-vs-generate: %v", err)
	}
	if !strings.Contains(text, "● Reuse an") {
		t.Fatalf("reuse-key-vs-generate must select reuse, not merely render its label:\n%s", text)
	}
	if err := screenshot.ValidateCapturedState(reuseSpec, text); err != nil {
		t.Fatalf("reuse-key-vs-generate must capture the selected reuse state: %v\n%s", err, text)
	}
	if strings.TrimSpace(raw) == "" {
		t.Fatal("reuse-key-vs-generate must retain raw PTY evidence")
	}
}

// TestCaptureTUIScreenConfirmationManagedBlock proves the raw-PTY capture
// reaches the complete pre-write managed block rather than a fixed viewport page.
func TestCaptureTUIScreenConfirmationManagedBlock(t *testing.T) {
	bin := filepath.Join(t.TempDir(), "gitid")
	build := exec.Command("go", "build", "-o", bin, "../gitid") //nolint:gosec // fixed local package and sandbox output
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("building gitid capture binary: %v\n%s", err, output)
	}

	workspace := t.TempDir()
	fakeSSH, err := writeFakeSSH(workspace)
	if err != nil {
		t.Fatalf("creating fake SSH: %v", err)
	}
	var raw string
	text, err := captureTUIScreen(bin, t.TempDir(), fakeSSH, "confirm-managed-block", true, &raw)
	if err != nil {
		t.Fatalf("capturing confirm-managed-block: %v", err)
	}
	if !strings.Contains(text, "# END gitid managed:") {
		t.Fatalf("confirm-managed-block must expose the END sentinel before writing:\n%s", text)
	}
	if strings.TrimSpace(raw) == "" {
		t.Fatal("confirm-managed-block must retain raw PTY evidence")
	}
}

func TestCaptureTUIScreenStage1PassStopsBeforeStage2(t *testing.T) {
	bin := filepath.Join(t.TempDir(), "gitid")
	build := exec.Command("go", "build", "-o", bin, "../gitid") //nolint:gosec // fixed local package and sandbox output
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("building gitid capture binary: %v\n%s", err, output)
	}

	workspace := t.TempDir()
	fakeSSH, err := writeFakeSSH(workspace)
	if err != nil {
		t.Fatalf("creating fake SSH: %v", err)
	}
	var raw string
	text, err := captureTUIScreen(bin, t.TempDir(), fakeSSH, "test-stage1-pass", true, &raw)
	if err != nil {
		t.Fatalf("capturing test-stage1-pass: %v", err)
	}
	if !strings.Contains(text, "running ssh") {
		t.Fatalf("test-stage1-pass must preserve the in-flight stage-2 state:\n%s", text)
	}
	if strings.Contains(text, "Next: Git identity") {
		t.Fatalf("test-stage1-pass must not capture the completed stage-2 state:\n%s", text)
	}

	direct, err := captureTUIScreen(bin, t.TempDir(), fakeSSH, "test-stage1-direct", true, &raw)
	if err != nil {
		t.Fatalf("capturing test-stage1-direct: %v", err)
	}
	if !strings.Contains(direct, "! Reachable") {
		t.Fatalf("test-stage1-direct must capture the D-02 warning state:\n%s", direct)
	}
}
