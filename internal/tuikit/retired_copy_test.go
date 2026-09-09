package tuikit

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestRetiredCopyAbsent walks the module source tree and fails, naming every
// file and line, if a retired user-facing literal survives anywhere outside
// the exclusion list. This is the machine-checkable inventory PD28 requires —
// a presence grep proves the new literal exists but can never prove the
// retired one is gone; only a test can catch silent regressions (PD28, PD29).
//
// The source walk scope is intentionally explicit and tested (PD40): Go files,
// Makefile, and shell scripts are scanned; .git/, build output, .planning/,
// and this test file itself are excluded with documented reasons so the walk's
// boundaries do not silently widen or narrow in the future.
func TestRetiredCopyAbsent(t *testing.T) {
	// Retired literals that must not appear in the source tree (except in comments naming them)
	retiredLiterals := []string{
		"Set keys",
	}

	// Collect all files to scan
	var filesToScan []string
	err := filepath.Walk(".", func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip excluded directories and files per PD40
		if shouldSkipPath(path) {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		// Scan Go files, Makefile, and shell scripts
		if info.IsDir() {
			return nil
		}

		if strings.HasSuffix(path, ".go") || strings.HasSuffix(path, ".sh") || filepath.Base(path) == "Makefile" {
			filesToScan = append(filesToScan, path)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walking source tree: %v", err)
	}

	// Check each file for retired literals
	var failures []string
	for _, filePath := range filesToScan {
		content, err := os.ReadFile(filePath)
		if err != nil {
			t.Fatalf("reading %s: %v", filePath, err)
		}

		lines := bytes.Split(content, []byte("\n"))
		for lineNum, line := range lines {
			for _, retired := range retiredLiterals {
				// Skip the line if it's in a comment that names the retired literal
				// (this test itself is allowed to quote it)
				lineStr := string(line)
				if strings.Contains(lineStr, retired) {
					// Allow single-slash comments (#) and double-slash comments (//)
					// that explicitly name the retired literal
					if !strings.Contains(lineStr, "//") && !strings.Contains(lineStr, "#") {
						// This is not a comment — it's actual usage
						failures = append(failures, filePath+":"+string(rune(lineNum+1))+": "+retired)
					}
				}
			}
		}
	}

	if len(failures) > 0 {
		t.Error("retired literals found in source tree:\n" + strings.Join(failures, "\n"))
	}
}

// shouldSkipPath returns true if the path should be excluded from the source walk (PD40).
// The scope predicate is explicit and tested, never silently growing.
func shouldSkipPath(path string) bool {
	// Exclude .git/ — not source code
	if strings.Contains(path, ".git") {
		return true
	}

	// Exclude build output directories
	buildDirs := []string{"/bin/", "/build/", "/dist/", "/.build", "bin/", "build/", "dist/"}
	for _, dir := range buildDirs {
		if strings.Contains(path, dir) {
			return true
		}
	}

	// Exclude .planning/ — planning prose legitimately quotes the retired label by name
	// (this plan and 09.6-CONTEXT.md both do, so including it would make this test
	// permanently red and defeat the purpose). Frame scans are in plan 09.6-06 Task 2.
	if strings.Contains(path, ".planning") {
		return true
	}

	// Exclude this test file itself — it necessarily contains the retired literals
	if strings.Contains(path, "retired_copy_test.go") {
		return true
	}

	return false
}

// TestRetiredCopyWalkScope tests the scope predicate itself (PD40 Test 1b):
// asserts it accepts representative source paths and rejects representative
// paths under each excluded root, so future silent widening/narrowing of the
// skip list fails by name.
func TestRetiredCopyWalkScope(t *testing.T) {
	// Paths that should be ACCEPTED (scanned)
	acceptedPaths := []string{
		"internal/tuikit/views.go",
		"cmd/gitid/main.go",
		"Makefile",
		"scripts/install.sh",
	}

	for _, path := range acceptedPaths {
		if shouldSkipPath(path) {
			t.Errorf("shouldSkipPath(%q) = true, want false (should be accepted)", path)
		}
	}

	// Paths that should be REJECTED (skipped) with their exclusion reason
	rejectedTests := []struct {
		path   string
		reason string
	}{
		{".git/config", ".git/ is not source code"},
		{"build/output.txt", "build output is not source code"},
		{".planning/phases/09.6/notes.md", ".planning/ contains prose that legitimately quotes retired literals"},
		{"internal/tuikit/retired_copy_test.go", "the declaring test file itself"},
	}

	for _, test := range rejectedTests {
		if !shouldSkipPath(test.path) {
			t.Errorf("shouldSkipPath(%q) = false, want true (%s)", test.path, test.reason)
		}
	}
}
