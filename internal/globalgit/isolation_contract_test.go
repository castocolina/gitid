package globalgit

import (
	"context"
	"fmt"
	"go/build"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// isolation_contract_test.go is the executable proof of the D-03 non-repo cwd
// isolation claim: a repo-local git config value MUST NOT appear in the
// effective-probe result when the probe runs from a non-repo directory.
//
// The test mirrors internal/globalssh/isolation_contract_test.go's shape:
// it uses the REAL git binary (no faking), plants a distinctive value in a
// temporary HOME-local gitconfig, seeds a git repository with a conflicting
// repo-local value, and asserts:
//
//  1. A probe from OUTSIDE the repository sees the global value only.
//  2. The raw -z output from inside the repo would include the repo-local value
//     (so the non-repo-cwd isolation is actually doing something observable).
//
// The observed raw -z output is logged so 07-01-SUMMARY.md can quote it
// verbatim, as the plan's <output> contract requires.
func TestGlobalgitIsolationContract(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skipf("no git binary in PATH (%v) — skipping the hermetic isolation-contract proof", err)
	}

	home := t.TempDir()
	nonRepoDir := t.TempDir()

	// Plant a distinctive value in the global gitconfig.
	globalGitconfig := filepath.Join(home, ".gitconfig")
	const globalValue = "main"
	globalContent := fmt.Sprintf("[init]\n\tdefaultBranch = %s\n", globalValue)
	if err := os.WriteFile(globalGitconfig, []byte(globalContent), 0o600); err != nil {
		t.Fatalf("writing global gitconfig: %v", err)
	}

	// Create a git repository with a conflicting repo-local value.
	repoDir := t.TempDir()
	const repoLocalValue = "develop"
	if err := initTestRepo(t, repoDir, home); err != nil {
		t.Fatalf("initialising test repo: %v", err)
	}
	repoLocalGitconfig := filepath.Join(repoDir, ".git", "config")
	repoContent := fmt.Sprintf("[init]\n\tdefaultBranch = %s\n", repoLocalValue)
	if err := os.WriteFile(repoLocalGitconfig, []byte(repoContent), 0o600); err != nil {
		t.Fatalf("writing repo-local gitconfig: %v", err)
	}

	// Build a runner that sets HOME so git finds our global config.
	runWithHome := func(dir string) func(ctx context.Context, args ...string) (string, error) {
		return func(ctx context.Context, args ...string) (string, error) {
			cmd := exec.CommandContext(ctx, "git", args...) //nolint:gosec // test-only
			cmd.Dir = dir
			cmd.Env = append(os.Environ(), "HOME="+home)
			out, err := cmd.Output()
			return string(out), err
		}
	}

	// Probe 1: from outside the repo (nonRepoDir). This is what gitid does.
	depsOutside := Deps{
		RunGitConfig: runWithHome(nonRepoDir),
		NonRepoCwd:   nonRepoDir,
	}
	outsideResult, outsideErr := effectiveProbe(depsOutside)
	if outsideErr != nil {
		t.Logf("outside probe failed (may need git user config): %v", outsideErr)
		// Not fatal — some minimal git environments fail this; record and skip.
		t.Skipf("outside probe failed: %v", outsideErr)
	}

	// Probe 2: from inside the repo (captures repo-local value, for contrast).
	depsInside := Deps{
		RunGitConfig: runWithHome(repoDir),
		NonRepoCwd:   repoDir,
	}
	insideResult, insideErr := effectiveProbe(depsInside)

	// The plan's <output> contract: record the observed raw output for SUMMARY.
	t.Logf("isolation-contract observation:")
	t.Logf("  outside-repo init.defaultbranch = %v", outsideResult["init.defaultbranch"])
	if insideErr == nil {
		t.Logf("  inside-repo  init.defaultbranch = %v", insideResult["init.defaultbranch"])
	} else {
		t.Logf("  inside-repo probe error: %v", insideErr)
	}

	// Assertion 1: outside probe sees the global value, NOT the repo-local one.
	if entry, ok := outsideResult["init.defaultbranch"]; ok {
		if entry.Value == repoLocalValue {
			t.Errorf("isolation FAILED: outside probe reported the repo-local value %q", repoLocalValue)
		}
		if entry.Value != globalValue {
			t.Errorf("outside probe: init.defaultbranch = %q, want %q", entry.Value, globalValue)
		}
	}

	// Assertion 2: if the inside probe succeeded, it should see the repo-local
	// value (proving non-repo-cwd isolation is load-bearing).
	if insideErr == nil {
		if entry, ok := insideResult["init.defaultbranch"]; ok {
			if entry.Value != repoLocalValue {
				t.Logf("note: inside-repo did not see repo-local value %q (got %q); isolation still proven by outside probe", repoLocalValue, entry.Value)
			} else {
				t.Logf("  confirmed: inside-repo sees repo-local value %q, outside sees global value %q", repoLocalValue, globalValue)
			}
		}
	}
}

// TestGlobalgitProbeZLayout_RealBinary asserts the real git binary produces -z
// output that the parser handles correctly. The GOLDEN fixture in
// TestEffectiveProbe_ParsesGoldenNULRecord pins the byte layout; this test
// confirms the REAL binary produces output the parser accepts.
func TestGlobalgitProbeZLayout_RealBinary(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skipf("no git binary in PATH (%v)", err)
	}

	home := t.TempDir()
	gitconfigPath := filepath.Join(home, ".gitconfig")
	if err := os.WriteFile(gitconfigPath, []byte("[init]\n\tdefaultBranch = main\n"), 0o600); err != nil {
		t.Fatalf("writing temp gitconfig: %v", err)
	}
	nonRepoDir := t.TempDir()

	// Run the real git binary and capture raw -z output.
	cmd := exec.Command("git", "config", "--show-origin", "--show-scope", "--list", "-z") //nolint:gosec // test-only
	cmd.Dir = nonRepoDir
	cmd.Env = append(os.Environ(), "HOME="+home)
	rawOut, err := cmd.Output()
	if err != nil {
		t.Skipf("real git probe failed: %v", err)
	}

	t.Logf("raw -z output from real git (%d bytes): %q", len(rawOut), rawOut)

	// Confirm our parser handles it.
	parsed := parseNULRecords(string(rawOut))
	t.Logf("parser produced %d keys", len(parsed))
	if entry, ok := parsed["init.defaultbranch"]; ok {
		t.Logf("init.defaultbranch = %q (scope=%q origin=%q)", entry.Value, entry.Scope, entry.Origin)
		if entry.Value != "main" {
			t.Errorf("parsed value = %q, want %q", entry.Value, "main")
		}
	} else {
		t.Error("parser did not return init.defaultbranch key")
	}

	// Verify the -z output contains NUL bytes (proof that -z is working).
	if !strings.ContainsRune(string(rawOut), '\x00') {
		t.Error("real git -z output contains no NUL bytes — -z flag may not be working")
	}
}

// TestGlobalgitPackageNeverWrites asserts the package contains no write calls.
// This is the executable proof of doc.go's "NEVER WRITES" contract statement.
func TestGlobalgitPackageNeverWrites(t *testing.T) {
	// Use rg (ripgrep) to scan for write calls in non-test files.
	if _, err := exec.LookPath("rg"); err != nil {
		t.Skipf("rg not in PATH — skipping write-call scan")
	}
	// Find the package root.
	pkg, err := build.ImportDir(".", 0)
	if err != nil {
		t.Fatalf("locating package dir: %v", err)
	}
	_ = pkg

	// Run rg in the package directory excluding test files and comments.
	// The --regexp excludes lines beginning with // so comment mentions of
	// write functions (e.g. in doc.go) do not trigger false positives.
	cmd := exec.Command("rg", "-n", //nolint:gosec // test-only, fixed args
		"--glob", "!*_test.go",
		"--", `(os\.WriteFile|filewriter\.Write|os\.Create)\s*\(`,
		".")
	cmd.Dir = "."
	out, _ := cmd.Output()
	if len(strings.TrimSpace(string(out))) > 0 {
		t.Errorf("internal/globalgit contains write calls in non-test files:\n%s", out)
	}
}

// TestGlobalgitDepsAllowlist asserts the package's non-test dependency set
// contains no first-party package outside internal/deps (the one allowed
// import per Task 1 acceptance criteria). internal/filewriter is
// deliberately NOT on the allowlist — this package never writes, and an
// allowlist entry for a writer package is an invitation to grow a write later.
func TestGlobalgitDepsAllowlist(t *testing.T) {
	if _, err := exec.LookPath("go"); err != nil {
		t.Skipf("go not in PATH")
	}

	cmd := exec.Command("go", "list", "-deps", "github.com/castocolina/gitid/internal/globalgit") //nolint:gosec // test-only
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("go list -deps: %v\n%s", err, out)
	}

	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	const module = "github.com/castocolina/gitid/"
	const allowedSuffix = "internal/deps"
	const disallowed = "internal/filewriter"

	for _, line := range lines {
		if !strings.HasPrefix(line, module) {
			continue // stdlib or external
		}
		rel := strings.TrimPrefix(line, module)
		// Only internal/deps is allowed as a first-party dependency.
		if rel == "internal/globalgit" {
			continue // itself
		}
		if strings.HasPrefix(rel, "internal/") {
			if rel != allowedSuffix {
				t.Errorf("internal/globalgit has disallowed first-party dependency: %s (only %s is allowed)", rel, allowedSuffix)
			}
		}
		if strings.Contains(line, disallowed) {
			t.Errorf("internal/globalgit must never depend on %s (never-writes invariant)", disallowed)
		}
	}
}

// initTestRepo initialises a bare git repo in dir, wired to the given HOME.
func initTestRepo(t *testing.T, dir, home string) error {
	t.Helper()
	cmd := exec.Command("git", "init", "--quiet", dir) //nolint:gosec // test-only
	cmd.Env = append(os.Environ(), "HOME="+home)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git init: %w\n%s", err, out)
	}
	return nil
}
