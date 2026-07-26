package tui

// deps_test.go — Tests for buildTUIDeleteDeps (Plan 06, D-16).
//
// Mirrors cmd/gitid/delete_test.go's missing-file no-op semantics for RemoveKeyFiles.
// The CANONICAL analog: cmd/gitid/buildDeleteDeps.RemoveKeyFiles routes both key and
// .pub through filewriter.BackupAndRemove; a missing file is a no-op (empty backup
// path, no error). This test exercises the same contract for buildTUIDeleteDeps.

import (
	"testing"
)

// TestBuildTUIDeleteDepsRemoveKeyFilesNoOp verifies that RemoveKeyFiles on a
// missing file is a no-op — it returns ("", "", nil) without error.
// This mirrors the cmd/gitid/delete.go buildDeleteDeps semantics (D-16, CR-02).
// Requirement: TUI-06/D-16 (buildTUIDeleteDeps mirrors buildDeleteDeps).
// Closes: Plan 06 (Task 2 additional verification).
func TestBuildTUIDeleteDepsRemoveKeyFilesNoOp(t *testing.T) {
	d := buildTUIDeleteDeps()
	if d.RemoveKeyFiles == nil {
		t.Fatal("RemoveKeyFiles must not be nil")
	}

	// Call RemoveKeyFiles on paths that do not exist.
	// Expected: no error (missing file is a no-op per filewriter.BackupAndRemove semantics).
	keyBak, pubBak, err := d.RemoveKeyFiles(
		"/tmp/gitid-test-nonexistent-key-abc123",
		"/tmp/gitid-test-nonexistent-key-abc123.pub",
	)
	if err != nil {
		t.Errorf("RemoveKeyFiles on missing files must be a no-op (no error); got: %v", err)
	}
	// Missing files produce empty backup paths (nothing to back up).
	if keyBak != "" {
		t.Errorf("RemoveKeyFiles on missing private key must return empty backup path; got %q", keyBak)
	}
	if pubBak != "" {
		t.Errorf("RemoveKeyFiles on missing public key must return empty backup path; got %q", pubBak)
	}
}

// TestBuildTUIDeleteDepsReadSSHMissingOK verifies that ReadSSH on a missing
// ~/.ssh/config returns ([]byte{}, nil) — same as buildDeleteDeps.
func TestBuildTUIDeleteDepsReadSSHMissingOK(t *testing.T) {
	// This test exercises the real buildTUIDeleteDeps().ReadSSH, which internally
	// reads from the actual ~/.ssh/config path. We cannot easily override it in a
	// unit test without modifying the wiring, but we CAN verify the no-error
	// contract by calling it directly and asserting it returns a non-nil byte slice
	// (whether empty or populated, a nil return would indicate a bug).
	d := buildTUIDeleteDeps()
	if d.ReadSSH == nil {
		t.Fatal("ReadSSH must not be nil")
	}
	data, err := d.ReadSSH()
	if err != nil {
		t.Errorf("ReadSSH must not error even if ~/.ssh/config does not exist; got: %v", err)
	}
	if data == nil {
		t.Error("ReadSSH must return non-nil []byte (empty slice is fine)")
	}
}
