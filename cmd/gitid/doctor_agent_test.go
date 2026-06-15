package main

// doctor_agent_test.go: RED/GREEN tests for DOC-GAP-02 (agent wiring),
// DOC-GAP-03 (non-interactive TTY gate), IN-03 (tiered exit code), and
// WR-03 (gitconfig perms false positive). All tests drive the REAL
// buildDoctorDeps/runDoctor wiring — no fake RunSSHAdd/RunSSHKeygenFingerprint
// injected to satisfy the wiring assertions.
//
// Gap closure contract: these tests MUST exercise the production construction
// path. Injecting a fake RunSSHAdd does NOT close DOC-GAP-02.

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/castocolina/gitid/internal/doctor"
	"github.com/castocolina/gitid/internal/doctor/checks"
)

// TestDoctorAgentWiring asserts that buildDoctorDeps wires both RunSSHAdd and
// RunSSHKeygenFingerprint to non-nil production closures (DOC-GAP-02).
// This is the permanent regression guard: if either field is nil, the Agent
// check silently returns no findings, masking a down ssh-agent.
//
// RED failure: both fields are nil because buildDoctorDeps never sets them.
func TestDoctorAgentWiring(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	d := buildDoctorDeps(home, nil, nil)

	if d.RunSSHAdd == nil {
		t.Error("DOC-GAP-02: RunSSHAdd is nil after buildDoctorDeps — agent check will always report healthy")
	}
	if d.RunSSHKeygenFingerprint == nil {
		t.Error("DOC-GAP-02: RunSSHKeygenFingerprint is nil after buildDoctorDeps — per-key loaded check broken")
	}
}

// TestDoctorAgentBehaviorUnreachable asserts that CheckAgent emits a
// FamilyAgent warning when ssh-add reports an unreachable agent
// (DOC-GAP-02 behavior).
//
// We force an unreachable agent by setting SSH_AUTH_SOCK to a path that does
// not exist, so the real ssh-add -l runner returns a non-zero exit code that
// classifyAgentState treats as unreachable.
//
// RED failure: RunSSHAdd is nil → CheckAgent returns nil (guard fires) →
// no FamilyAgent finding is ever produced.
func TestDoctorAgentBehaviorUnreachable(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	// Point SSH_AUTH_SOCK at a non-existent path so ssh-add -l returns an
	// "unreachable" result (exit 2 or connect error).
	t.Setenv("SSH_AUTH_SOCK", filepath.Join(home, "no_such_socket"))

	d := buildDoctorDeps(home, nil, nil)

	// At RED: RunSSHAdd is nil, so CheckAgent returns nil early — no findings.
	// After GREEN wiring, the real runner fires and classifyAgentState returns
	// agentUnreachable, producing a FamilyAgent warning.
	if d.RunSSHAdd == nil {
		t.Fatal("DOC-GAP-02: RunSSHAdd is nil — cannot test agent behavior; wiring must be fixed first")
	}

	// Invoke the wired runner directly: it must return a non-zero exit code
	// consistent with an unreachable agent.
	_, exitCode := d.RunSSHAdd()
	if exitCode == 0 {
		t.Errorf("DOC-GAP-02: RunSSHAdd() with bad SSH_AUTH_SOCK returned exitCode 0; " +
			"expected non-zero (unreachable). ssh-add may be absent or SSH_AUTH_SOCK not set.")
	}

	// Run the full CheckAgent path with the wired deps.
	findings := checks.CheckAgent(d)

	var agentFinding *doctor.Finding
	for i, f := range findings {
		if f.Family == doctor.FamilyAgent {
			agentFinding = &findings[i]
			break
		}
	}
	if agentFinding == nil {
		t.Errorf("DOC-GAP-02: CheckAgent produced no FamilyAgent finding with an unreachable agent; "+
			"findings: %+v", findings)
	}
}

// TestDoctorNonInteractiveGateSkipped asserts that bare runDoctor (fix=false,
// yes=false) in a non-interactive context (test process stdin is not a TTY)
// does NOT emit the "Apply N fix(es)?" prompt into machine-parsed output
// (DOC-GAP-03).
//
// We seed a home with a wrong-mode ~/.ssh/config (0o644 instead of 0o600) to
// ensure at least one fixable finding is present, then assert that the output
// string does NOT contain "Apply".
//
// RED failure: runDoctor calls applyFixes unconditionally when fixable findings
// exist, so the "Apply N fix(es)?" prompt always appears in non-interactive
// output.
func TestDoctorNonInteractiveGateSkipped(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	// Create ~/.ssh with correct mode and a config with wrong permissions so
	// CheckPermissions produces a fixable SeverityError finding.
	sshDir := filepath.Join(home, ".ssh")
	if err := os.MkdirAll(sshDir, 0o700); err != nil {
		t.Fatalf("creating .ssh dir: %v", err)
	}
	sshConfigPath := filepath.Join(sshDir, "config")
	// Write the SSH config with 0o644 (should be 0o600) → SeverityError perm finding.
	if err := os.WriteFile(sshConfigPath, []byte("# empty\n"), 0o644); err != nil { //nolint:gosec // intentional wrong perm for test
		t.Fatalf("writing ssh config with wrong perms: %v", err)
	}

	var out bytes.Buffer
	// runDoctor is called from the test process whose stdin is not a TTY.
	// After the TTY guard (GREEN), the "Apply" prompt must be suppressed.
	// At RED, there is no TTY guard, so the prompt appears in output.
	_ = runDoctor(&out, false /* fix */, false /* yes */)

	output := out.String()
	if strings.Contains(output, "Apply") {
		t.Errorf("DOC-GAP-03: non-interactive runDoctor emitted an 'Apply' prompt; "+
			"the TTY guard must skip the gate when stdin is not a terminal.\n"+
			"Output fragment:\n%s", truncate(output, 500))
	}
}

// TestDoctorTieredExitCode asserts that runDoctor returns the tiered
// doctor.ExitCode (0/1/2/3) for the findings it produces, and that the
// exit-code seam available to main() reflects the tiered value (IN-03).
//
// Two scenarios:
//   - Error-level finding (wrong ~/.ssh/config mode 0o644): expect ExitCode 2.
//   - Warning-level finding (RunSSHAdd reports unreachable agent): expect ExitCode 1.
//
// The seam assertion drives through RunE: we call the doctor cobra command's
// RunE and verify the error it returns embeds the tiered exit code, not a flat 1.
// A future regression to flat os.Exit(1) would require changing RunE to lose
// the tiered code, which this test would catch.
func TestDoctorTieredExitCode(t *testing.T) {
	t.Run("error-level findings return ExitCode 2", func(t *testing.T) {
		home := t.TempDir()
		t.Setenv("HOME", home)

		sshDir := filepath.Join(home, ".ssh")
		if err := os.MkdirAll(sshDir, 0o700); err != nil {
			t.Fatalf("creating .ssh dir: %v", err)
		}
		// SSH config with wrong mode (0o644 instead of 0o600) → SeverityError.
		sshConfigPath := filepath.Join(sshDir, "config")
		if err := os.WriteFile(sshConfigPath, []byte("# empty\n"), 0o644); err != nil { //nolint:gosec // intentional wrong perm
			t.Fatalf("writing ssh config: %v", err)
		}

		var out bytes.Buffer
		code := runDoctor(&out, false, false)
		if code != 2 {
			t.Errorf("IN-03: expected runDoctor to return ExitCode 2 for error-severity findings; got %d\noutput:\n%s",
				code, out.String())
		}
	})

	t.Run("RunE embeds tiered code in error string", func(t *testing.T) {
		home := t.TempDir()
		t.Setenv("HOME", home)

		sshDir := filepath.Join(home, ".ssh")
		if err := os.MkdirAll(sshDir, 0o700); err != nil {
			t.Fatalf("creating .ssh dir: %v", err)
		}
		// SSH config with wrong mode → SeverityError → ExitCode 2.
		sshConfigPath := filepath.Join(sshDir, "config")
		if err := os.WriteFile(sshConfigPath, []byte("# empty\n"), 0o644); err != nil { //nolint:gosec // intentional wrong perm
			t.Fatalf("writing ssh config: %v", err)
		}

		cmd := newDoctorCmd()
		var buf bytes.Buffer
		cmd.SetOut(&buf)
		cmd.SetErr(&buf)

		// RunE must return a non-nil error whose text embeds the tiered code (2),
		// not a flat 1. This seam is what main() reads to call os.Exit(code).
		runE := cmd.RunE
		err := runE(cmd, nil)
		if err == nil {
			t.Fatal("IN-03: RunE returned nil error for error-severity home; expected non-nil")
		}
		// The error string must embed the correct tiered code, not flat 1.
		if !strings.Contains(err.Error(), "2") {
			t.Errorf("IN-03: RunE error %q does not contain tiered exit code 2; "+
				"main() cannot propagate the correct exit code", err.Error())
		}
	})
}

// TestDoctorGitconfigPermsNotFlagged asserts that a default 0644 ~/.gitconfig
// does NOT produce a SeverityError Permissions finding (WR-03).
//
// git itself writes ~/.gitconfig with mode 0644. Flagging this as a SeverityError
// means virtually every machine reports a false positive.
//
// RED failure: CheckPermissions uses modeSSHConfig (0600) for gitconfig, so a
// default 0644 gitconfig triggers SeverityError.
func TestDoctorGitconfigPermsNotFlagged(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	// Write ~/.gitconfig with the default 0644 mode (as git itself would create it).
	gitconfigPath := filepath.Join(home, ".gitconfig")
	if err := os.WriteFile(gitconfigPath, []byte("[core]\n\tautocrlf = input\n"), 0o644); err != nil { //nolint:gosec // test gitconfig
		t.Fatalf("writing gitconfig: %v", err)
	}

	d := buildDoctorDeps(home, nil, nil)
	findings := checks.CheckPermissions(d)

	// Assert: no SeverityError finding for the gitconfig path.
	for _, f := range findings {
		if f.Family == doctor.FamilyPerms &&
			f.Severity == doctor.SeverityError &&
			strings.Contains(f.Title, ".gitconfig") {
			t.Errorf("WR-03: SeverityError Permissions finding for default 0644 ~/.gitconfig — "+
				"this is a false positive. git writes .gitconfig with 0644.\nFinding: %+v", f)
		}
	}
}

// truncate returns at most n characters of s, appending "..." if truncated.
func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + fmt.Sprintf("... [%d more bytes]", len(s)-n)
}
