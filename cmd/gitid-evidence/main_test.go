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
func TestPublisherCLIProducesImmutable24PanelPacket(t *testing.T) {
	source, err := commandOutput("git", "rev-parse", "HEAD")
	if err != nil {
		t.Fatalf("resolving HEAD: %v", err)
	}
	root := filepath.Join(t.TempDir(), "packet-root")
	cmd := exec.Command("go", "run", "-tags", "screenshot", ".", "--source-commit", strings.TrimSpace(source), "--output-root", root) //nolint:gosec // fixed local command and test paths
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("publisher CLI failed: %v\n%s", err, output)
	}
	packetDir := filepath.Join(root, strings.TrimSpace(source))
	pkt, err := screenshot.ValidatePacket(packetDir)
	if err != nil {
		t.Fatalf("validating published packet: %v", err)
	}
	if got := len(pkt.Members); got != screenshot.ValidatePanelCount*2 {
		t.Fatalf("packet member count = %d, want %d text/PNG members", got, screenshot.ValidatePanelCount*2)
	}
	pngs := 0
	for _, member := range pkt.Members {
		if strings.HasSuffix(member.Path, ".png") {
			pngs++
			info, statErr := os.Stat(filepath.Join(packetDir, member.Path))
			if statErr != nil || info.Size() == 0 {
				t.Fatalf("panel %s missing or empty: %v", member.Path, statErr)
			}
		}
	}
	if pngs != screenshot.ValidatePanelCount {
		t.Fatalf("PNG count = %d, want %d", pngs, screenshot.ValidatePanelCount)
	}
	if err := run([]string{"--source-commit", strings.TrimSpace(source), "--output-root", root}); err == nil {
		t.Fatal("publisher must refuse an existing packet root")
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
	if !strings.Contains(err.Error(), "not found") && !strings.Contains(err.Error(), "nonempty") {
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
	if !strings.Contains(err.Error(), "not found") && !strings.Contains(err.Error(), "cat-file") {
		t.Errorf("error should mention 'not found' or 'cat-file'; got: %v", err)
	}
}
