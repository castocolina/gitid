//go:build e2e

package e2e

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"testing"
)

// packageBin is the shared binary path built once for the entire test package
// via buildOnce. Using a package-level binary avoids running `go build` inside
// t.TempDir() (which would put the Go module cache there, causing permission
// errors during cleanup because go.mod files are read-only).
//
// realHome is captured at package init time — before any test can call
// t.Setenv("HOME", sandbox). This preserves the original GOPATH derivation
// even when tests change HOME to a sandbox.
var (
	buildOnce  sync.Once
	packageBin string
	buildErr   error
	realHome   = os.Getenv("HOME") // captured at init, before any t.Setenv
)

// SandboxHome creates a hermetic HOME directory and sets HOME to it via
// t.Setenv so it is automatically restored after the test.
// All files the binary writes (including ~/.ssh and ~/.gitconfig) land there.
func SandboxHome(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	return home
}

// BuildBinary compiles the gitid binary once per test package run and returns
// its path. Subsequent calls return the same cached binary path without
// recompiling (fast and deterministic). The binary is placed in os.MkdirTemp
// (not t.TempDir) so the Go module cache does not land inside a test-managed
// directory — this avoids cleanup permission errors (go.mod is read-only).
func BuildBinary(t *testing.T) string {
	t.Helper()
	buildOnce.Do(func() {
		dir, err := os.MkdirTemp("", "gitid-e2e-*")
		if err != nil {
			buildErr = err
			return
		}
		bin := filepath.Join(dir, "gitid")
		cmd := exec.Command("go", "build", "-o", bin, "./cmd/gitid")
		cmd.Dir = repoRoot(t)
		// Restore the original HOME so `go build` derives GOPATH from the real
		// home (not from a sandbox). This prevents go.mod files being written
		// into t.TempDir() which would cause cleanup permission errors.
		cmd.Env = append(os.Environ(), "HOME="+realHome)
		if combined, berr := cmd.CombinedOutput(); berr != nil {
			buildErr = fmt.Errorf("%w\n%s", berr, combined)
			_ = os.RemoveAll(dir)
			return
		}
		packageBin = bin
	})
	if buildErr != nil {
		t.Fatalf("BuildBinary: go build failed: %v", buildErr)
	}
	if packageBin == "" {
		t.Fatal("BuildBinary: binary path is empty after build")
	}
	return packageBin
}

// FakeSSHDir writes a mode-switching fake ssh script to a temp dir and sets
// GITID_FAKE_SSH_MODE to mode. The caller prepends the returned dir to PATH via
// cmd.Env so the child gitid binary resolves the fake ssh instead of /usr/bin/ssh.
//
// The script is a static string literal — never constructed from user input and
// never passed to a shell interpreter from Go code (D-20, gosec G-204 safe).
//
// Modes:
//
//	pass    — emits "successfully authenticated" banner (tester.PASS outcome)
//	denied  — emits "Permission denied (publickey)" (tester.ReachableNotUploaded)
//	timeout — emits a connect-timeout line (tester.Failure)
//
// The script also handles "ssh -G <alias>" by emitting a fixture ssh -G block
// so the Resolved dep works correctly in E2E tests.
func FakeSSHDir(t *testing.T, mode string) string {
	t.Helper()
	dir := t.TempDir()

	// Static string literal — not constructed from user input, never exec'd via sh -c.
	// The -Q branch must come first (ProbeKeyTypes), then -G (Resolved), then
	// the GITID_FAKE_SSH_MODE-dispatched connection test.
	const script = "#!/bin/sh\n" +
		"case \"$1\" in\n" +
		"  -Q)\n" +
		"    echo \"ssh-ed25519\"\n" +
		"    echo \"ssh-rsa\"\n" +
		"    echo \"ecdsa-sha2-nistp256\"\n" +
		"    exit 0\n" +
		"    ;;\n" +
		"  -G)\n" +
		"    echo \"user git\"\n" +
		"    echo \"hostname ssh.github.com\"\n" +
		"    echo \"port 443\"\n" +
		"    echo \"identitiesonly yes\"\n" +
		"    echo \"identityfile /tmp/fake/.ssh/id_ed25519_testid\"\n" +
		"    exit 0\n" +
		"    ;;\n" +
		"esac\n" +
		"case \"$GITID_FAKE_SSH_MODE\" in\n" +
		"  pass)\n" +
		"    echo \"Hi user! You've successfully authenticated, but GitHub does not provide shell access.\"\n" +
		"    exit 1\n" +
		"    ;;\n" +
		"  denied)\n" +
		"    echo \"git@ssh.github.com: Permission denied (publickey).\"\n" +
		"    exit 255\n" +
		"    ;;\n" +
		"  timeout)\n" +
		"    echo \"ssh: connect to host ssh.github.com port 443: Operation timed out\"\n" +
		"    exit 255\n" +
		"    ;;\n" +
		"  *)\n" +
		"    echo \"git@ssh.github.com: Permission denied (publickey).\"\n" +
		"    exit 255\n" +
		"    ;;\n" +
		"esac\n"

	scriptPath := filepath.Join(dir, "ssh")
	if err := os.WriteFile(scriptPath, []byte(script), 0o700); err != nil { //nolint:gosec // test-only static script (G306)
		t.Fatalf("FakeSSHDir: writing fake ssh: %v", err)
	}
	t.Setenv("GITID_FAKE_SSH_MODE", mode)
	return dir
}

// repoRoot walks up from the test working directory until it finds a directory
// containing go.mod, which marks the repository root.
func repoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("repoRoot: Getwd: %v", err)
	}
	for {
		if _, statErr := os.Stat(filepath.Join(dir, "go.mod")); statErr == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatalf("repoRoot: go.mod not found walking up from %s", dir)
		}
		dir = parent
	}
}
