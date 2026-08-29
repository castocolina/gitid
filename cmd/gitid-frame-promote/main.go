// gitid-frame-promote is the ONE reproducible command (09-07-PLAN.md Task 1,
// review R14) that promotes Phase 9 upload-surface PTY frames from the
// gitignored capture directory (tmp/ui-frames/, e2e/ui_pty_e2e_test.go's
// saveFrame) into the tracked D-09 approved-baseline directory
// (.planning/phases/09-upload-credentials-assist/ui-frames/), recording
// machine-checked provenance for each promoted frame so a later frame from a
// DIFFERENT run can never be mistaken for the reviewed one.
//
// Run the PTY suite first (captures land in tmp/ui-frames/), then:
//
//	go run ./cmd/gitid-frame-promote
//
// Every promoted frame's row in ui-frames/PROVENANCE.md records: the state
// ID, the frame filename, the producing test function, the shim mode it was
// captured under, the fixed capture geometry, the source commit
// (`git rev-parse HEAD`), and the SHA-256 of the frame's content.
// TestUploadFrameProvenanceMatches (cmd/gitid/gate_visual_regression_test.go)
// verifies every row against the committed frame's real hash in both
// directions.
package main

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

// promotionEntry pairs one captured frame with the provenance facts the
// promoted PROVENANCE.md row must record — the registry-to-frame mapping
// the plan requires stays explicit and reviewable here, rather than
// inferred from filenames.
type promotionEntry struct {
	stateID  string
	frame    string // filename in tmp/ui-frames/, without the .txt suffix
	test     string
	shimMode string
	geometry string
}

// phase9Frames is the fixed set of Phase 9 upload-surface frames this tool
// promotes — 09-07-PLAN.md Task 1's twelve new PTY tests plus the two
// pre-existing plan-09-02 tracer tests that already cover an Approved Base
// State row.
var phase9Frames = []promotionEntry{
	{"upload-checkbox-ready", "create-flow-upload-autonomous-github", "TestCreateFlow_UploadAutonomousGitHubTracer", "gh ok", "100x30"},
	{"upload-checkbox-unauth", "create-flow-upload-checkbox-unauth", "TestCreateFlow_UploadCheckboxUnauthState", "gh auth-fail", "100x30"},
	{"upload-checkbox-disabled", "create-flow-upload-checkbox-disabled", "TestCreateFlow_UploadCheckboxDisabledState", "deny shim (no gh)", "100x30"},
	{"upload-checkbox-tab-and-click", "create-flow-upload-checkbox-tab-and-click", "TestCreateFlow_UploadCheckboxTabAndClickReachable", "gh ok", "100x30"},
	{"upload-manual-fallback", "create-flow-upload-partial-scope", "TestCreateFlow_UploadPartialScopeShowsBothRows", "gh scope-fail-signing", "100x30"},
	{"upload-already-complete", "create-flow-upload-already-complete", "TestCreateFlow_UploadAlreadyCompleteCollapsesToOneLine", "gh inventory-both", "100x30"},
	{"upload-omitted", "create-flow-reachable-not-uploaded-evidence", "TestCreateFlow_TestStageReachableNotUploaded", "fake ssh denied, no gh", "100x30"},
	{"register-key-modal", "identity-manager-register-key-modal-runs", "TestIdentityManager_RegisterKeyModalRuns", "gh ok", "100x30"},
	{"register-key-modal-manual-fallback", "identity-manager-register-key-modal-manual-fallback", "TestIdentityManager_RegisterKeyModalManualFallback", "gh auth-fail", "100x30"},
	{"register-key-modal-u-key", "identity-manager-register-key-modal-u-key", "TestIdentityManager_RegisterKeyModalOpensWithU", "gh ok", "100x30"},
	{"rotate-delete-offer-default", "identity-manager-rotate-delete-offer-default", "TestIdentityManager_RotateDeleteOfferDefaultsToLeave", "gh delete-ok + inventory", "100x30"},
	{"rotate-delete-offer-delete", "identity-manager-rotate-delete-offer-delete", "TestIdentityManager_RotateDeleteOfferDeletesOnExplicitChoice", "gh delete-ok + inventory", "100x30"},
	{"rotate-delete-offer-absent", "identity-manager-rotate-delete-offer-absent", "TestIdentityManager_RotateDeleteOfferAbsentWhenInventoryFails", "gh inventory-fail", "100x30"},
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "gitid-frame-promote: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	root, err := repoRoot()
	if err != nil {
		return fmt.Errorf("resolving repo root: %w", err)
	}
	srcDir := filepath.Join(root, "tmp", "ui-frames")
	dstDir := filepath.Join(root, ".planning", "phases", "09-upload-credentials-assist", "ui-frames")
	if err := os.MkdirAll(dstDir, 0o750); err != nil {
		return fmt.Errorf("creating %s: %w", dstDir, err)
	}
	commit, err := gitHead(root)
	if err != nil {
		return fmt.Errorf("resolving source commit: %w", err)
	}

	rows, err := promoteFrames(srcDir, dstDir, phase9Frames, commit, func(format string, a ...any) { fmt.Printf(format, a...) })
	if err != nil {
		return err
	}

	if err := writeProvenance(filepath.Join(dstDir, "README.md"), rows); err != nil {
		return fmt.Errorf("writing PROVENANCE table: %w", err)
	}
	fmt.Printf("promoted %d frame(s); provenance written to %s\n", len(rows), filepath.Join(dstDir, "README.md"))
	return nil
}

// promoteFrames copies every entry's captured frame from srcDir into dstDir
// and returns each promoted row for the PROVENANCE table, or an error
// naming every missing capture. progress is called once per promoted frame
// (nil is fine — tests pass nil to stay silent).
//
// WR-13: read and validate EVERY source frame before writing anything. The
// old version wrote each tracked baseline as it went and only checked for
// missing captures after the loop finished — so a partial capture run (one
// PTY test skipped, one source frame absent) rewrote SOME approved
// baselines while leaving the rest at their previous commit's content, and
// the caller never regenerated README.md at all: a baseline directory whose
// frames and provenance table disagree, and whose non-rewritten frames can
// be mistaken for having come from THIS run. Nothing is written to dstDir
// until every entry has a real source file in srcDir.
func promoteFrames(srcDir, dstDir string, frames []promotionEntry, commit string, progress func(format string, a ...any)) ([]string, error) {
	entries := append([]promotionEntry(nil), frames...)
	sort.Slice(entries, func(i, j int) bool { return entries[i].stateID < entries[j].stateID })

	contents := make(map[string][]byte, len(entries))
	var missing []string
	for _, e := range entries {
		srcPath := filepath.Join(srcDir, e.frame+".txt")
		content, err := os.ReadFile(srcPath) //nolint:gosec // fixed, tool-owned path
		if err != nil {
			missing = append(missing, e.frame)
			continue
		}
		contents[e.frame] = content
	}
	if len(missing) > 0 {
		return nil, fmt.Errorf("missing captures in %s (run the PTY suite first): %v", srcDir, missing)
	}

	var rows []string
	for _, e := range entries {
		content := contents[e.frame]
		dstPath := filepath.Join(dstDir, e.frame+".txt")
		if err := os.WriteFile(dstPath, content, 0o644); err != nil { //nolint:gosec // tracked repo file, not a secret
			return nil, fmt.Errorf("writing %s: %w", dstPath, err)
		}
		sum := sha256.Sum256(content)
		rows = append(rows, fmt.Sprintf("| %s | %s.txt | %s | %s | %s | %s | %s |",
			e.stateID, e.frame, e.test, e.shimMode, e.geometry, commit, hex.EncodeToString(sum[:])))
		if progress != nil {
			progress("promoted %-45s <- tmp/ui-frames/%s.txt\n", e.frame+".txt", e.frame)
		}
	}
	return rows, nil
}

func writeProvenance(path string, rows []string) error {
	var b strings.Builder
	b.WriteString("# Phase 9 upload-surface approved PTY frames\n\n")
	b.WriteString("These are D-09's fresh approved captures for exactly the amended upload\n")
	b.WriteString("screens — the Phase 9 visual-regression baseline every gate-visual-regression\n")
	b.WriteString("run compares the real binary against. Every frame is captured at the fixed\n")
	b.WriteString("100x30 geometry, driven by a real PTY session with raw keystrokes against the\n")
	b.WriteString("compiled `gitid` binary (never a unit/wiring-test substitute — ONESHOT.md rule 8).\n\n")
	b.WriteString("Promoted by `go run ./cmd/gitid-frame-promote` — never hand-copied. Re-run that\n")
	b.WriteString("command after any PTY test change; it overwrites this table and every frame in\n")
	b.WriteString("this directory from a fresh `tmp/ui-frames/` capture.\n\n")
	b.WriteString("## Provenance\n\n")
	b.WriteString("| State ID | Frame | Producing test | Shim mode | Geometry | Source commit | SHA-256 |\n")
	b.WriteString("|---|---|---|---|---|---|---|\n")
	for _, r := range rows {
		b.WriteString(r + "\n")
	}
	return os.WriteFile(path, []byte(b.String()), 0o644) //nolint:gosec // tracked repo file, not a secret
}

func repoRoot() (string, error) {
	out, err := exec.Command("git", "rev-parse", "--show-toplevel").Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

func gitHead(root string) (string, error) {
	cmd := exec.Command("git", "rev-parse", "HEAD")
	cmd.Dir = root
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}
