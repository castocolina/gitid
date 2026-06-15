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
// no managed identities, no bad perms) prints the all-clear summary with
// exit code 0 or 1 (info findings from missing optional dirs are acceptable).
func TestDoctorCleanAllClear(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	var out bytes.Buffer
	code := runDoctor(&out, false, false)
	output := out.String()

	// A clean temp home has no critical or error findings; code should be 0 or 1.
	if code > 1 {
		t.Errorf("runDoctor on clean home returned exit code %d, want 0 or 1; output:\n%s", code, output)
	}
	// The all-clear message appears when exit code is 0.
	if code == 0 && !strings.Contains(output, "doctor: all checks passed") {
		t.Errorf("exit code 0 but missing 'doctor: all checks passed' in output:\n%s", output)
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
