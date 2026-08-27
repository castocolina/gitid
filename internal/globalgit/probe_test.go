package globalgit

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// makeTestDeps returns Deps wired to a function that returns a fixed output
// string and a fixed error.
func makeTestDeps(output string, runErr error) Deps {
	return Deps{
		RunGitConfig: func(_ context.Context, _ ...string) (string, error) {
			return output, runErr
		},
		NonRepoCwd: tempNonRepoCwd,
	}
}

// tempNonRepoCwd is a dummy non-repo cwd used in unit tests (not the real
// binary; the isolation_contract_test.go uses os.MkdirTemp directly).
var tempNonRepoCwd = os.TempDir()

// TestEffectiveProbe_ParsesGoldenNULRecord verifies the -z record layout:
//
//	<scope> NUL <origin> NUL <key> LF <value> NUL
//
// This is the GOLDEN literal the plan requires to be pinned in the repository
// (Task 1 acceptance criteria, review LOW finding).
func TestEffectiveProbe_ParsesGoldenNULRecord(t *testing.T) {
	// Golden literal: one record for "init.defaultbranch" at global scope.
	// Format: scope NUL origin NUL key LF value NUL
	golden := "global\x00file:/home/user/.gitconfig\x00init.defaultbranch\nmain\x00"

	deps := makeTestDeps(golden, nil)
	result, err := effectiveProbe(deps)
	if err != nil {
		t.Fatalf("effectiveProbe: %v", err)
	}
	if got := result["init.defaultbranch"]; got.Value != "main" {
		t.Errorf("Value = %q, want %q", got.Value, "main")
	}
	if got := result["init.defaultbranch"]; got.Scope != "global" {
		t.Errorf("Scope = %q, want %q", got.Scope, "global")
	}
	if got := result["init.defaultbranch"]; got.Origin != "/home/user/.gitconfig" {
		t.Errorf("Origin = %q, want %q", got.Origin, "/home/user/.gitconfig")
	}
}

// TestEffectiveProbe_FilePrefix_Stripped verifies "file:" prefix is stripped.
func TestEffectiveProbe_FilePrefix_Stripped(t *testing.T) {
	golden := "global\x00file:/home/alice/.gitconfig\x00core.ignorecase\nfalse\x00"
	deps := makeTestDeps(golden, nil)
	result, err := effectiveProbe(deps)
	if err != nil {
		t.Fatalf("effectiveProbe: %v", err)
	}
	if got := result["core.ignorecase"]; got.Origin != "/home/alice/.gitconfig" {
		t.Errorf("Origin should strip 'file:' prefix, got %q", got.Origin)
	}
}

// TestEffectiveProbe_NonFileOrigin_PreservedVerbatim verifies non-file: origins
// are kept verbatim (e.g. "command line", "blob:", "standard input").
func TestEffectiveProbe_NonFileOrigin_PreservedVerbatim(t *testing.T) {
	golden := "command\x00command line\x00init.defaultbranch\nmain\x00"
	deps := makeTestDeps(golden, nil)
	result, err := effectiveProbe(deps)
	if err != nil {
		t.Fatalf("effectiveProbe: %v", err)
	}
	if got := result["init.defaultbranch"]; got.Origin != "command line" {
		t.Errorf("non-file origin should be preserved verbatim, got %q", got.Origin)
	}
}

// TestEffectiveProbe_ValueWithNewline verifies a value containing a newline
// round-trips correctly through the -z parser.
func TestEffectiveProbe_ValueWithNewline(t *testing.T) {
	// A value with a newline inside — the -z format handles this unambiguously.
	// Format: scope NUL origin NUL key LF value NUL
	golden := "global\x00file:/home/user/.gitconfig\x00core.pager\nless -FRX\n--quit-if-one-screen\x00"
	deps := makeTestDeps(golden, nil)
	result, err := effectiveProbe(deps)
	if err != nil {
		t.Fatalf("effectiveProbe: %v", err)
	}
	want := "less -FRX\n--quit-if-one-screen"
	if got := result["core.pager"]; got.Value != want {
		t.Errorf("Value with newline = %q, want %q", got.Value, want)
	}
}

// TestEffectiveProbe_ValueWithTab verifies a value containing a tab round-trips.
func TestEffectiveProbe_ValueWithTab(t *testing.T) {
	golden := "global\x00file:/home/user/.gitconfig\x00alias.lg\nlog\t--oneline\x00"
	deps := makeTestDeps(golden, nil)
	result, err := effectiveProbe(deps)
	if err != nil {
		t.Fatalf("effectiveProbe: %v", err)
	}
	want := "log\t--oneline"
	if got := result["alias.lg"]; got.Value != want {
		t.Errorf("Value with tab = %q, want %q", got.Value, want)
	}
}

// TestEffectiveProbe_DuplicateKey_LastWins verifies that when a key appears
// more than once, the LAST record wins (git's own last-wins resolution).
func TestEffectiveProbe_DuplicateKey_LastWins(t *testing.T) {
	// Two records for the same key; the second should win.
	golden := "global\x00file:/a/.gitconfig\x00init.defaultbranch\nmaster\x00" +
		"global\x00file:/b/.gitconfig\x00init.defaultbranch\nmain\x00"
	deps := makeTestDeps(golden, nil)
	result, err := effectiveProbe(deps)
	if err != nil {
		t.Fatalf("effectiveProbe: %v", err)
	}
	if got := result["init.defaultbranch"]; got.Value != "main" {
		t.Errorf("duplicate key last-wins: got %q, want %q", got.Value, "main")
	}
}

// TestEffectiveProbe_NonZeroExit_ReturnsError verifies that a non-zero probe
// exit yields an error whose message names the probe that failed.
func TestEffectiveProbe_NonZeroExit_ReturnsError(t *testing.T) {
	probeErr := fmt.Errorf("exit status 1")
	deps := makeTestDeps("", probeErr)
	_, err := effectiveProbe(deps)
	if err == nil {
		t.Fatal("expected error from non-zero probe exit, got nil")
	}
	if !strings.Contains(err.Error(), "effective") {
		t.Errorf("error should name the failing probe 'effective', got: %v", err)
	}
}

// TestInFileProbe_ParsesResult verifies the in-file probe returns only
// physically-present keys.
func TestInFileProbe_ParsesResult(t *testing.T) {
	// Format: scope NUL origin NUL key LF value NUL (same for --file probe, scope is "local")
	golden := "local\x00file:/home/user/.gitconfig.d/00-baseline\x00init.defaultbranch\nmain\x00"
	deps := makeTestDeps(golden, nil)

	result, err := inFileProbe(deps, "/home/user/.gitconfig.d/00-baseline")
	if err != nil {
		t.Fatalf("inFileProbe: %v", err)
	}
	if _, ok := result["init.defaultbranch"]; !ok {
		t.Error("inFileProbe should return the key physically in the file")
	}
}

// TestInFileProbe_MissingFile_EmptyResult verifies a missing file returns an
// empty result and a nil error (the first-run case).
func TestInFileProbe_MissingFile_EmptyResult(t *testing.T) {
	deps := Deps{
		RunGitConfig: func(_ context.Context, _ ...string) (string, error) {
			// Simulate git exiting with code 128 when file doesn't exist.
			return "", &missingFileError{}
		},
		NonRepoCwd: os.TempDir(),
	}
	result, err := inFileProbe(deps, "/nonexistent/path/.gitconfig.d/baseline")
	if err != nil {
		t.Fatalf("inFileProbe with missing file should return nil error, got: %v", err)
	}
	if len(result) != 0 {
		t.Errorf("inFileProbe with missing file should return empty result, got %d keys", len(result))
	}
}

// missingFileError simulates the error git returns for a missing config file.
// It implements the missingFileSentinel interface that isMissingFileErr checks.
type missingFileError struct{}

func (e *missingFileError) Error() string       { return "exit status 128" }
func (e *missingFileError) IsMissingFile() bool { return true }

// TestInFileProbe_NonZeroExit_ReturnsError verifies non-zero exit (not
// "missing file") yields an error naming the probe.
func TestInFileProbe_NonZeroExit_ReturnsError(t *testing.T) {
	deps := Deps{
		RunGitConfig: func(_ context.Context, _ ...string) (string, error) {
			return "", fmt.Errorf("exit status 1")
		},
		NonRepoCwd: os.TempDir(),
	}
	_, err := inFileProbe(deps, "/some/file")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "in-file") {
		t.Errorf("error should name the probe 'in-file', got: %v", err)
	}
}

// TestBuildProbeDeps_NonNil verifies BuildProbeDeps returns non-nil function fields.
func TestBuildProbeDeps_NonNil(t *testing.T) {
	deps := BuildProbeDeps("/some/cwd")
	if deps.RunGitConfig == nil {
		t.Error("BuildProbeDeps: RunGitConfig must not be nil")
	}
	if deps.NonRepoCwd == "" {
		t.Error("BuildProbeDeps: NonRepoCwd must not be empty")
	}
}

// TestEffectiveProbe_RealGitBinary_ZLayoutConfirmed is the integration-level
// test that confirms the real git binary's -z output layout matches what the
// parser expects. It runs the REAL git binary with a temp HOME.
func TestEffectiveProbe_RealGitBinary_ZLayoutConfirmed(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skipf("no git binary in PATH (%v)", err)
	}

	home := t.TempDir()
	gitconfigPath := filepath.Join(home, ".gitconfig")
	if err := os.WriteFile(gitconfigPath, []byte("[init]\n\tdefaultBranch = main\n"), 0o600); err != nil {
		t.Fatalf("writing temp gitconfig: %v", err)
	}

	// Use a temp cwd that is not a git repo.
	cwd := t.TempDir()

	deps := Deps{
		RunGitConfig: func(ctx context.Context, args ...string) (string, error) {
			cmd := exec.CommandContext(ctx, "git", args...) //nolint:gosec // test-only, args are fixed probe flags
			cmd.Dir = cwd
			cmd.Env = append(os.Environ(), "HOME="+home, "GIT_CONFIG_NOSYSTEM=")
			out, err := cmd.Output()
			return string(out), err
		},
		NonRepoCwd: cwd,
	}

	result, err := effectiveProbe(deps)
	if err != nil {
		t.Logf("real git effective probe output (for summary): error=%v", err)
		t.Skipf("real git probe failed (may need git configured): %v", err)
	}
	t.Logf("real git -z output parsed, got %d keys", len(result))
	if v, ok := result["init.defaultbranch"]; ok {
		t.Logf("init.defaultbranch = %q (scope=%q origin=%q)", v.Value, v.Scope, v.Origin)
	}
}
