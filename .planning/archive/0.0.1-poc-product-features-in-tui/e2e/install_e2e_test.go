//go:build e2e

package e2e

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
)

// TestInstall_PathFeedback verifies that `gitid doctor` surfaces the binary
// install path and a PATH membership hint (D-17). This covers both surface A
// (gitid self-report via doctor) and the requirement that the user can determine
// where gitid is installed without consulting the Makefile.
//
// This test is RED until FIX-INSTALL-01 is implemented (the current doctor does
// not surface install-path or PATH-membership findings).
func TestInstall_PathFeedback(t *testing.T) {
	home := SandboxHome(t)
	bin := BuildBinary(t)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	var stdout, stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, bin, "doctor")
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	cmd.Env = append(os.Environ(), "HOME="+home)

	// doctor may exit non-zero. We only assert output content.
	_ = cmd.Run()

	combined := stdout.String() + stderr.String()

	// Assert: doctor output mentions the binary path (FIX-INSTALL-01, D-17 surface A).
	// After implementation, doctor includes an "Install" or "Binary" finding with the
	// resolved binary path. Accept any of several plausible labels.
	hasPathInfo := strings.Contains(combined, "install") ||
		strings.Contains(combined, "Install") ||
		strings.Contains(combined, "binary") ||
		strings.Contains(combined, "Binary") ||
		strings.Contains(combined, "PATH")

	if !hasPathInfo {
		t.Errorf("doctor must report install path or PATH membership;\nstdout:\n%s\nstderr:\n%s",
			stdout.String(), stderr.String())
	}

	// Assert: the doctor output mentions a concrete path to the gitid binary
	// (surface A: os.Executable() result or go env GOPATH/bin hint).
	// Accept either the bin path we built or any GOPATH/bin reference.
	hasBinPath := strings.Contains(combined, bin) ||
		strings.Contains(combined, "gitid") && strings.Contains(combined, "/bin/")

	if !hasBinPath {
		t.Errorf("doctor must include a concrete install path for gitid;\nstdout:\n%s\nstderr:\n%s",
			stdout.String(), stderr.String())
	}
}

// TestInstall_MakeInstallOutput verifies that `make install` prints a resolved
// install path and a PATH hint when the install dir is not already on PATH
// (D-17, Surface B: Makefile echo).
//
// This test is RED until FIX-INSTALL-01 is implemented (the current Makefile
// install target does not print install-path or PATH hints).
func TestInstall_MakeInstallOutput(t *testing.T) {
	root := repoRoot(t)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	var stdout, stderr bytes.Buffer
	// Run `make install` from the repo root. The output should include the
	// resolved install path and a PATH export hint.
	cmd := exec.CommandContext(ctx, "make", "install")
	cmd.Dir = root
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	cmd.Env = os.Environ()

	if err := cmd.Run(); err != nil {
		t.Fatalf("make install failed: %v\nstdout: %s\nstderr: %s", err, stdout.String(), stderr.String())
	}

	combined := stdout.String() + stderr.String()

	// Assert: output contains the resolved install path.
	if !strings.Contains(combined, "installed:") && !strings.Contains(combined, "install path") {
		t.Errorf("make install must print install path;\ncombined output:\n%s", combined)
	}

	// Assert: output contains a PATH hint (either "PATH: OK" or "export PATH=...").
	if !strings.Contains(combined, "PATH") {
		t.Errorf("make install must print PATH membership hint;\ncombined output:\n%s", combined)
	}
}
