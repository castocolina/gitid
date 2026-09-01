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

// TestInstall_MakeInstallOutput verifies that `make install` prints a resolved
// install path and a PATH hint when the install dir is not already on PATH
// (D-17, Surface B: Makefile echo).
//
// Phase 3 note: this test was RESTORED after the POC archive wave (plan
// 03-03). The rest of the original install_e2e_test.go drove `gitid doctor`,
// a POC Cobra command that Phase 3 archived, so that half was correctly
// retired with the surface it exercised. THIS half is different: it asserts
// the behavior of the `install` MAKEFILE TARGET, which survives Phase 3
// untouched. Archiving it would have silently dropped coverage of live
// functionality — the target still echoes both the resolved install path and
// the PATH membership hint this test pins.
func TestInstall_MakeInstallOutput(t *testing.T) {
	root := repoRoot(t)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second*ciTimeoutMultiplier())
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
