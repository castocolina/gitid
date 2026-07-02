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
)

// seedOverlappingGitconfig writes two gitid managed includeIf blocks with the
// given gitdir values into ~/.gitconfig in the sandbox home. This sets up the
// overlap scenario without going through the full create-flow.
func seedOverlappingGitconfig(t *testing.T, home, nameA, gitdirA, nameB, gitdirB string) {
	t.Helper()
	gitconfigPath := filepath.Join(home, ".gitconfig")
	content := "" +
		"# BEGIN gitid managed: " + nameA + "\n" +
		"[includeIf \"gitdir:" + gitdirA + "\"]\n" +
		"\tpath = " + filepath.Join(home, ".gitconfig.d", nameA) + "\n" +
		"# END gitid managed: " + nameA + "\n" +
		"\n" +
		"# BEGIN gitid managed: " + nameB + "\n" +
		"[includeIf \"gitdir:" + gitdirB + "\"]\n" +
		"\tpath = " + filepath.Join(home, ".gitconfig.d", nameB) + "\n" +
		"# END gitid managed: " + nameB + "\n"
	if err := os.WriteFile(gitconfigPath, []byte(content), 0o644); err != nil { //nolint:gosec // test-only sandbox file (G306)
		t.Fatalf("seedOverlappingGitconfig: writing ~/.gitconfig: %v", err)
	}
}

// TestDoctor_OverlapWarning_IdenticalGitdir verifies that `gitid doctor` warns
// when two identities have the same gitdir match value (D-14, identical gitdir
// overlap). Severity = warning; exit code stays in the warning tier (D-15).
//
// This test is RED until DOC-08 is implemented (the current doctor does not check
// for overlapping includeIf conditions).
func TestDoctor_OverlapWarning_IdenticalGitdir(t *testing.T) {
	home := SandboxHome(t)
	bin := BuildBinary(t)

	// Seed: two identities with identical gitdir ~/git/shared/.
	seedOverlappingGitconfig(t, home,
		"identA", "~/git/shared/",
		"identB", "~/git/shared/",
	)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	var stdout, stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, bin, "doctor")
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	cmd.Env = append(os.Environ(), "HOME="+home)

	// doctor may exit non-zero (warning tier). That is acceptable (D-15).
	_ = cmd.Run()

	combined := stdout.String() + stderr.String()

	// Assert: doctor warns about overlapping gitdir.
	if !strings.Contains(combined, "overlap") && !strings.Contains(combined, "Overlap") &&
		!strings.Contains(combined, "ambig") && !strings.Contains(combined, "identical") {
		t.Errorf("doctor must warn about identical gitdir overlap;\nstdout:\n%s\nstderr:\n%s",
			stdout.String(), stderr.String())
	}
}

// TestDoctor_OverlapWarning_NestedGitdir verifies that `gitid doctor` warns when
// one identity's gitdir is a prefix (parent) of another's gitdir, creating an
// ambiguous-capture overlap (D-14, nested gitdir overlap).
//
// This test is RED until DOC-08 is implemented.
func TestDoctor_OverlapWarning_NestedGitdir(t *testing.T) {
	home := SandboxHome(t)
	bin := BuildBinary(t)

	// Seed: identA matches ~/git/ (broad), identB matches ~/git/personal/ (narrow).
	// ~/git/ subsumes ~/git/personal/ → nested overlap.
	seedOverlappingGitconfig(t, home,
		"identA", "~/git/",
		"identB", "~/git/personal/",
	)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	var stdout, stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, bin, "doctor")
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	cmd.Env = append(os.Environ(), "HOME="+home)

	// doctor may exit non-zero (warning tier). That is acceptable (D-15).
	_ = cmd.Run()

	combined := stdout.String() + stderr.String()

	// Assert: doctor warns about nested/prefix gitdir overlap.
	if !strings.Contains(combined, "overlap") && !strings.Contains(combined, "Overlap") &&
		!strings.Contains(combined, "nested") && !strings.Contains(combined, "subsume") &&
		!strings.Contains(combined, "ambig") {
		t.Errorf("doctor must warn about nested gitdir overlap;\nstdout:\n%s\nstderr:\n%s",
			stdout.String(), stderr.String())
	}
}
