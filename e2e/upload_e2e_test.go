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

// seedIdentityForUpload seeds a minimal gitid identity so there is a public key
// to operate on during upload tests. It runs `gitid identity add` with fake-ssh
// in pass mode and returns the home directory (already set by SandboxHome).
func seedIdentityForUpload(t *testing.T, home, bin string) {
	t.Helper()
	fakeSSHDir := FakeSSHDir(t, "pass")

	stdin := strings.NewReader(strings.Join([]string{
		"1",             // mode: create-new
		"testid",        // identity name
		"Test User",     // git user.name
		"t@example.com", // git user.email
		"github",        // provider
		"",              // alias (default)
		"",              // hostname (default)
		"",              // port (default)
		"",              // match strategy (default: gitdir)
		"",              // gitdir value (default)
		"",              // passphrase (empty)
	}, "\n") + "\n")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	var stdout, stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, bin, "identity", "add")
	cmd.Stdin = stdin
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	cmd.Env = append(os.Environ(),
		"HOME="+home,
		"GITID_FAKE_SSH_MODE=pass",
		"PATH="+fakeSSHDir+":"+os.Getenv("PATH"),
	)
	if err := cmd.Run(); err != nil {
		t.Fatalf("seedIdentityForUpload: gitid identity add failed: %v\nstdout: %s\nstderr: %s",
			err, stdout.String(), stderr.String())
	}
}

// TestUpload_GHOk verifies that when `gh` is present on PATH and authenticated,
// `gitid identity copy testid --upload-keys --yes` invokes the gh upload flow
// and reports success ("--type authentication" or "Added SSH key") in output.
//
// This test is RED — the --upload-keys flag on `gitid identity copy` does not yet exist.
// Wave 1 Plan 04 (05.7-04) turns this GREEN.
func TestUpload_GHOk(t *testing.T) {
	home := SandboxHome(t)
	bin := BuildBinary(t)

	seedIdentityForUpload(t, home, bin)

	ghDir := FakeGHDir(t, "ok")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	var stdout, stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, bin, "identity", "copy", "testid",
		"--upload-keys", "--yes")
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	cmd.Env = append(os.Environ(),
		"HOME="+home,
		"PATH="+ghDir+":"+os.Getenv("PATH"),
		"GITID_FAKE_GH_MODE=ok",
	)

	if err := cmd.Run(); err != nil {
		t.Fatalf("gitid identity copy --upload-keys failed: %v\nstdout: %s\nstderr: %s",
			err, stdout.String(), stderr.String())
	}

	combined := stdout.String() + stderr.String()
	// Assert: gh upload invocation evidence appears in output.
	if !strings.Contains(combined, "--type authentication") &&
		!strings.Contains(combined, "Added SSH key") {
		t.Errorf("expected gh upload invocation in output;\ngot:\n%s", combined)
	}
}

// TestUpload_GHAuthFail verifies that when `gh` is present on PATH but not
// authenticated, `gitid identity copy testid --upload-keys --yes` falls back to
// showing manual upload instructions instead of attempting an upload.
//
// This test is RED — the --upload-keys flag on `gitid identity copy` does not yet exist.
// Wave 1 Plan 04 (05.7-04) turns this GREEN.
func TestUpload_GHAuthFail(t *testing.T) {
	home := SandboxHome(t)
	bin := BuildBinary(t)

	seedIdentityForUpload(t, home, bin)

	ghDir := FakeGHDir(t, "auth-fail")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	var stdout, stderr bytes.Buffer
	// Command may exit non-zero when gh auth fails; what we test is the fallback text.
	cmd := exec.CommandContext(ctx, bin, "identity", "copy", "testid",
		"--upload-keys", "--yes")
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	cmd.Env = append(os.Environ(),
		"HOME="+home,
		"PATH="+ghDir+":"+os.Getenv("PATH"),
		"GITID_FAKE_GH_MODE=auth-fail",
	)
	_ = cmd.Run()

	combined := stdout.String() + stderr.String()
	// Assert: manual fallback instructions appear in output.
	// The feature must show manual upload steps when gh is present but not authed.
	if !strings.Contains(combined, "github.com/settings/ssh") &&
		!strings.Contains(combined, "manual") &&
		!strings.Contains(combined, "Upload") {
		t.Errorf("expected manual-upload fallback text in output for auth-fail;\ngot:\n%s", combined)
	}
}

// TestUpload_GHNotPresent verifies that when gh is not present on PATH,
// `gitid identity copy testid --upload-keys --yes` does NOT show an
// assisted-upload section in output (only manual instructions).
//
// This test is RED — the --upload-keys flag on `gitid identity copy` does not yet exist.
// Wave 1 Plan 04 (05.7-04) turns this GREEN.
func TestUpload_GHNotPresent(t *testing.T) {
	home := SandboxHome(t)
	bin := BuildBinary(t)

	seedIdentityForUpload(t, home, bin)

	// Use a tempdir with NO gh or glab binary to simulate absent tool.
	emptyDir := t.TempDir()

	// Keep only standard system dirs that don't have gh/glab.
	systemPath := "/usr/bin:/bin:/usr/sbin:/sbin"

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	var stdout, stderr bytes.Buffer
	// Command may exit non-zero; what we test is the ABSENCE of assisted-upload text.
	pubPath := filepath.Join(home, ".ssh", "id_ed25519_testid.pub")
	cmd := exec.CommandContext(ctx, bin, "identity", "copy", "testid",
		"--upload-keys", "--yes")
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	cmd.Env = append(os.Environ(),
		"HOME="+home,
		"PATH="+emptyDir+":"+systemPath,
	)
	_ = pubPath // referenced to keep import; will be used by the real implementation
	_ = cmd.Run()

	combined := stdout.String() + stderr.String()
	// Assert: the output does NOT contain assisted-upload section markers.
	// When gh/glab is absent the tool must only show manual instructions.
	if strings.Contains(combined, "Upload to gh") ||
		strings.Contains(combined, "Press u to upload") {
		t.Errorf("assisted-upload section must NOT appear when gh/glab absent;\ngot:\n%s", combined)
	}
}
