package main

import (
	"os"
	"path/filepath"
	"testing"
)

// skipUnlessUnreadableFilesEnforced skips the calling test only when a
// mode-0000 regular file is readable by this process. The probe is the
// premise itself, not an euid check, so it also covers CAP_DAC_OVERRIDE
// without uid 0 and can never skip on a runner where mode 0000 is enforced.
// The motivating case is the fedora container CI job, which runs as root.
// internal/filewriter TestWriteRestoreOnError is the directory-permission
// precedent (it skips on euid 0); this helper is for regular files.
func skipUnlessUnreadableFilesEnforced(t *testing.T) {
	t.Helper()
	probe := filepath.Join(t.TempDir(), "unreadable-probe")
	if err := os.WriteFile(probe, []byte("probe"), 0o000); err != nil {
		t.Fatalf("write mode-0000 probe: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(probe, 0o600) })
	if _, err := os.ReadFile(probe); err == nil { //nolint:gosec // test-owned temp file; the read is the premise probe (G304)
		t.Skipf("unreadable-file premise cannot hold here: a mode-0000 file is readable by this process (euid=%d; root or CAP_DAC_OVERRIDE bypasses file permissions) — this test still runs on the non-root CI check matrix", os.Geteuid())
	}
}
