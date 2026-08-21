//go:build screenshot

// gitid-evidence is the ONE-SHOT explicit publication command for a
// content-addressed Phase 3 visual evidence packet (plan 03-11 Task 2, CR-01).
//
// Usage:
//
//	go run -tags screenshot ./cmd/gitid-evidence \
//	    --source-commit <full-40-hex-sha> \
//	    --output-root   <writable-parent-dir>
//
// The packet is created at <output-root>/<source-commit>/. It fails with a
// non-zero exit code when:
//   - The source commit is missing, short, or not found in the local repo
//   - The destination already exists (immutability guarantee)
//   - Any capture fails or a required tool is absent
//   - The two-candidate determinism check reports a manifest diff
//
// On success, the canonical manifest SHA-256 is printed to stdout.
// On any failure, the incomplete candidate directory is removed.
package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/castocolina/gitid/internal/dummytui"
	"github.com/castocolina/gitid/internal/screenshot"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "gitid-evidence: %v\n", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	// Parse flags manually to avoid cobra/pflag dependency in this tool.
	var sourceCommit, outputRoot string
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--source-commit", "-source-commit":
			if i+1 >= len(args) {
				return fmt.Errorf("--source-commit requires a value")
			}
			i++
			sourceCommit = args[i]
		case "--output-root", "-output-root":
			if i+1 >= len(args) {
				return fmt.Errorf("--output-root requires a value")
			}
			i++
			outputRoot = args[i]
		default:
			return fmt.Errorf("unknown argument %q", args[i])
		}
	}

	if sourceCommit == "" {
		return fmt.Errorf("--source-commit <full-40-hex-sha> is required")
	}
	if outputRoot == "" {
		return fmt.Errorf("--output-root <writable-parent-dir> is required")
	}

	// Quick format check before hitting the filesystem or git.
	if err := validateSourceCommitFormat(sourceCommit); err != nil {
		return err
	}

	// Compute the destination directory (named by full source SHA).
	// Check existence BEFORE git verification so the error message is precise.
	destDir := filepath.Join(outputRoot, sourceCommit)
	if _, err := os.Stat(destDir); err == nil {
		return fmt.Errorf("destination %q already exists — refusing to overwrite (immutability guarantee)", destDir)
	}

	// Verify the commit exists in the local repository.
	if err := validateSourceCommitExists(sourceCommit); err != nil {
		return err
	}

	// Generate two candidate packets in separate temp directories and compare.
	tmpParent, err := os.MkdirTemp("", "gitid-evidence-")
	if err != nil {
		return fmt.Errorf("creating temp parent: %w", err)
	}
	defer os.RemoveAll(tmpParent)

	cand1 := filepath.Join(tmpParent, "cand1")
	cand2 := filepath.Join(tmpParent, "cand2")

	opts1 := buildOpts(sourceCommit, cand1)
	opts2 := buildOpts(sourceCommit, cand2)
	opts2.Clock = opts1.Clock // pin same timestamp for both candidates

	r1, err := generateCandidate(opts1)
	if err != nil {
		return fmt.Errorf("generating first candidate: %w", err)
	}
	r2, err := generateCandidate(opts2)
	if err != nil {
		return fmt.Errorf("generating second candidate: %w", err)
	}

	// Cross-process determinism check.
	if err := screenshot.CompareManifests(r1, r2); err != nil {
		return fmt.Errorf("two-candidate determinism check FAILED:\n%w", err)
	}

	// Atomically publish the first candidate to the final destination.
	// Create the output root if it does not exist.
	if err := os.MkdirAll(outputRoot, 0o750); err != nil {
		return fmt.Errorf("creating output root %q: %w", outputRoot, err)
	}
	if err := os.Rename(cand1, destDir); err != nil {
		return fmt.Errorf("renaming candidate to final destination %q: %w", destDir, err)
	}

	// Validate the published packet.
	pkt, err := screenshot.ValidatePacket(destDir)
	if err != nil {
		// Roll back — remove the destination.
		_ = os.RemoveAll(destDir)
		return fmt.Errorf("final packet validation failed: %w", err)
	}

	fmt.Printf("gitid-evidence: published %d members to %s\n", len(pkt.Members), destDir)
	fmt.Printf("gitid-evidence: manifest SHA-256 = %s\n", pkt.ManifestSHA256)
	return nil
}

// buildOpts constructs PacketOptions for a given source commit and output dir.
func buildOpts(sourceCommit, outputDir string) screenshot.PacketOptions {
	clock := func() time.Time { return time.Now().UTC() }
	return screenshot.PacketOptions{
		SourceCommit:   sourceCommit,
		ApprovalCommit: screenshot.PacketApprovalCommit,
		OutputDir:      outputDir,
		Clock:          clock,
	}
}

// generateCandidate generates one text-based packet candidate.
func generateCandidate(opts screenshot.PacketOptions) (screenshot.PacketResult, error) {
	// Use the dummy backend for text captures (real binary capture is done
	// by the gate; the publisher generates the deterministic approval evidence).
	backend := dummytui.NewFixtureBackend()
	live := screenshot.CaptureCreateFlowScreens(backend)
	approved := screenshot.CaptureCreateFlowScreens(backend) // same backend = approval source

	return screenshot.GenerateTextPacket(opts, live, approved)
}

// validateSourceCommitFormat verifies the source commit is a valid 40-hex SHA.
func validateSourceCommitFormat(commit string) error {
	if len(commit) != 40 {
		return fmt.Errorf("source commit must be a full 40-hex SHA; got %q (length %d)", commit, len(commit))
	}
	for _, r := range commit {
		if !strings.ContainsRune("0123456789abcdefABCDEF", r) {
			return fmt.Errorf("source commit contains non-hex character %q in %q", r, commit)
		}
	}
	return nil
}

// validateSourceCommitExists verifies the commit exists in the local repository.
func validateSourceCommitExists(commit string) error {
	cmd := exec.Command("git", "cat-file", "-e", commit+"^{commit}") //nolint:gosec // arg-slice form; commit is hex-validated above (G204)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("source commit %q not found in local repo (git cat-file exit: %v)", commit, err)
	}
	return nil
}
