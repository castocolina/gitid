package main

import (
	"bytes"
	"strings"
	"testing"
)

// TestDoctorCmdRegistered verifies that `gitid doctor` is a registered top-level
// command on the root (not nested under a subgroup).
func TestDoctorCmdRegistered(t *testing.T) {
	root := newRootCmd()
	found := false
	for _, cmd := range root.Commands() {
		if cmd.Use == "doctor" {
			found = true
			break
		}
	}
	if !found {
		t.Error("newRootCmd() does not have a 'doctor' command registered")
	}
}

// TestDoctorRenderGrouped verifies that runDoctor produces output containing
// the grouped family headers (at minimum Permissions must appear).
func TestDoctorRenderGrouped(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	var out bytes.Buffer
	code := runDoctor(&out, false, false)
	if code < 0 || code > 3 {
		t.Errorf("runDoctor returned invalid exit code %d (want 0-3)", code)
	}

	output := out.String()
	if !strings.Contains(output, "=== Permissions ===") {
		t.Errorf("expected '=== Permissions ===' in output; got:\n%s", output)
	}
}

// TestDoctorCleanAllClear verifies that a clean temp home (no SSH config,
// no managed identities, no bad perms) runs without crashing and produces
// a valid exit code (0-3). A clean home will report baseline errors because
// the baseline has not been set up, which is correct behavior (CheckBaseline
// is now real, not a stub). The test checks that the output contains the
// expected grouped report structure and that no critical/panic occurs.
func TestDoctorCleanAllClear(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	var out bytes.Buffer
	code := runDoctor(&out, false, false)
	output := out.String()

	// Valid exit codes are 0-3; any other value indicates a bug.
	if code < 0 || code > 3 {
		t.Errorf("runDoctor on clean home returned invalid exit code %d (want 0-3); output:\n%s", code, output)
	}
	// The report must include at least the Baseline section.
	if !strings.Contains(output, "=== Baseline ===") {
		t.Errorf("expected '=== Baseline ===' in output; got:\n%s", output)
	}
	// A clean temp home has no SSH identity permission findings (no keys present).
	// Critical findings (e.g. exposed private key) should not appear on a fresh home.
	if code == 3 {
		t.Errorf("runDoctor on clean home returned critical exit code 3 (unexpected); output:\n%s", output)
	}
}

// TestDoctorYesRequiresFix verifies that --yes without --fix returns an error
// containing "doctor: --yes requires --fix".
func TestDoctorYesRequiresFix(t *testing.T) {
	root := newRootCmd()
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetErr(&buf)
	root.SetArgs([]string{"doctor", "--yes"})

	err := root.Execute()
	if err == nil {
		t.Fatal("expected error when --yes without --fix, got nil")
	}
	if !strings.Contains(err.Error(), "--yes requires --fix") {
		t.Errorf("expected error to contain '--yes requires --fix', got: %v", err)
	}
}
