package tester

import (
	"os"
	"os/exec"
	"strings"
	"testing"
)

// TestResolvedViaCommandMatchesResolvedViaArgv is the TEST-01 "shown command ==
// run command" contract for stage 2: the displayed string is built from the SAME
// argument slice ResolvedVia executes, not a hand-retyped literal. Comparing
// against resolvedViaArgs (the shared builder) means any future flag change to
// the real invocation moves both sides together or fails here.
func TestResolvedViaCommandMatchesResolvedViaArgv(t *testing.T) {
	const (
		configPath = "/tmp/gitid-stage/config"
		keyPath    = "/tmp/gitid-stage/id_ed25519_work"
		alias      = "work.github.com"
	)

	want := exec.Command("ssh", resolvedViaArgs(configPath, keyPath, alias, "")...).String() //nolint:gosec // arg-slice form for cmd.String() comparison; not executed
	got := ResolvedViaCommand(configPath, keyPath, alias, "")
	if got != want {
		t.Errorf("ResolvedViaCommand mismatch\n got: %q\nwant: %q", got, want)
	}
}

// TestResolvedViaCommandShape asserts the stage-2 command carries the staged
// config and the alias, but NO explicit -i — the connectivity invocation must
// rely on the config's IdentityFile, which is what the stage-2 proof tests.
// The `ssh -G` resolution call (which takes no -i) is NOT rendered here.
func TestResolvedViaCommandShape(t *testing.T) {
	got := ResolvedViaCommand("/tmp/cfg", "/tmp/key", "work.github.com", "")
	for _, want := range []string{
		"-F /tmp/cfg",
		"-o IdentitiesOnly=yes",
		"-o BatchMode=yes",
		"-T git@work.github.com",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("ResolvedViaCommand() = %q, must contain %q", got, want)
		}
	}
	if strings.Contains(got, "-i /tmp/key") {
		t.Errorf("ResolvedViaCommand must NOT pin -i; the staged config supplies the key; got %q", got)
	}
	if strings.Contains(got, " -G ") {
		t.Errorf("ResolvedViaCommand must render the connectivity call, not the -G call; got %q", got)
	}
}

// TestResolvedViaUsesSharedArgBuilder is the structural half of the parity
// guarantee (Pitfall 7): both the display helper and the executing function must
// read their argv from resolvedViaArgs. A future inline re-build inside
// ResolvedVia would silently re-open the drift this plan closes, so it is
// asserted against the source rather than left to review.
func TestResolvedViaUsesSharedArgBuilder(t *testing.T) {
	src, err := os.ReadFile("tester.go")
	if err != nil {
		t.Fatalf("reading tester.go: %v", err)
	}
	body := functionBody(t, string(src), "func ResolvedVia(")
	if !strings.Contains(body, "resolvedViaArgs(") {
		t.Errorf("ResolvedVia must build its argv via the shared resolvedViaArgs builder; body:\n%s", body)
	}
	// The `ssh -G` resolution call legitimately keeps its own inline `-F` args
	// (it takes no -i); only the CONNECTIVITY argv must come from the builder,
	// so the marker asserted here is one of its exclusive options.
	if strings.Contains(body, `"IdentitiesOnly=yes"`) {
		t.Errorf("ResolvedVia must not re-build its connectivity argv inline; body:\n%s", body)
	}
	cmdBody := functionBody(t, string(src), "func ResolvedViaCommand(")
	if !strings.Contains(cmdBody, "resolvedViaArgs(") {
		t.Errorf("ResolvedViaCommand must use the shared resolvedViaArgs builder; body:\n%s", cmdBody)
	}
	if strings.Contains(cmdBody, "CombinedOutput") || strings.Contains(cmdBody, "execRunner") {
		t.Errorf("ResolvedViaCommand is display-only and must never execute ssh; body:\n%s", cmdBody)
	}
}

// ---------------------------------------------------------------------------
// CR-07 — ResolvedViaGCommand and ValidateResolvedConfig
// ---------------------------------------------------------------------------

// TestResolvedViaGCommandShape verifies that ResolvedViaGCommand produces the
// ssh -G probe command — distinct from the connectivity command — using -F for
// the staged config path and -G for the alias (CR-07).
func TestResolvedViaGCommandShape(t *testing.T) {
	got := ResolvedViaGCommand("/tmp/staged-cfg", "work.github.com")
	for _, want := range []string{
		"-F /tmp/staged-cfg",
		"-G work.github.com",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("ResolvedViaGCommand() = %q, must contain %q", got, want)
		}
	}
	// Must not be the connectivity command.
	if strings.Contains(got, "-T git@") {
		t.Errorf("ResolvedViaGCommand must produce the -G probe, not the connectivity -T call; got %q", got)
	}
	if strings.Contains(got, "-o IdentitiesOnly=yes") {
		t.Errorf("ResolvedViaGCommand must not carry connectivity options; got %q", got)
	}
}

// TestValidateResolvedConfigMismatchedUser proves that a resolved config with
// the wrong User value is rejected by ValidateResolvedConfig (CR-07).
func TestValidateResolvedConfigMismatchedUser(t *testing.T) {
	rc := ResolvedConfig{
		User: "notgit", Hostname: "ssh.github.com", Port: "443",
		IdentitiesOnly: "yes", IdentityFiles: []string{"/k"},
	}
	if err := ValidateResolvedConfig(rc, ExpectedResolution{
		User: "git", Hostname: "ssh.github.com", Port: "443",
		IdentitiesOnly: "yes", ExpectedKeyPath: "/k",
	}); err == nil {
		t.Fatal("mismatched User must be rejected (CR-07)")
	}
}

// TestValidateResolvedConfigEmptyOutput proves that an all-zero ResolvedConfig
// (e.g. from empty ssh -G output) is rejected when any expected field is set
// (CR-07: fail closed on empty/truncated output).
func TestValidateResolvedConfigEmptyOutput(t *testing.T) {
	if err := ValidateResolvedConfig(ResolvedConfig{}, ExpectedResolution{
		User: "git", Hostname: "ssh.github.com", Port: "443",
		IdentitiesOnly: "yes", ExpectedKeyPath: "/k",
	}); err == nil {
		t.Fatal("empty ssh -G output must be rejected (CR-07)")
	}
}

// TestValidateResolvedConfigKeyNotFirst proves that when the expected key is
// listed SECOND (rather than first) in IdentityFiles, the validator rejects the
// resolution — the earlier key would shadow it (CR-07).
func TestValidateResolvedConfigKeyNotFirst(t *testing.T) {
	rc := ResolvedConfig{
		User: "git", Hostname: "ssh.github.com", Port: "443",
		IdentitiesOnly: "yes",
		IdentityFiles:  []string{"/home/.ssh/id_ed25519", "/home/.ssh/id_ed25519_acme"},
	}
	if err := ValidateResolvedConfig(rc, ExpectedResolution{
		User: "git", Hostname: "ssh.github.com", Port: "443",
		IdentitiesOnly: "yes", ExpectedKeyPath: "/home/.ssh/id_ed25519_acme",
	}); err == nil {
		t.Fatal("expected key not first in IdentityFiles must be rejected (CR-07)")
	}
}

// TestValidateResolvedConfigAllMatch proves the positive path: a correctly
// matched resolved config returns nil from the validator (CR-07).
func TestValidateResolvedConfigAllMatch(t *testing.T) {
	rc := ResolvedConfig{
		User: "git", Hostname: "ssh.github.com", Port: "443",
		IdentitiesOnly: "yes", IdentityFiles: []string{"/home/.ssh/id_ed25519_acme"},
	}
	if err := ValidateResolvedConfig(rc, ExpectedResolution{
		User: "git", Hostname: "ssh.github.com", Port: "443",
		IdentitiesOnly: "yes", ExpectedKeyPath: "/home/.ssh/id_ed25519_acme",
	}); err != nil {
		t.Errorf("all-matching resolved config must return nil: %v", err)
	}
}

// functionBody returns the source text from the given func declaration up to the
// next top-level declaration, so assertions are scoped to one function.
func functionBody(t *testing.T, src, decl string) string {
	t.Helper()
	start := strings.Index(src, decl)
	if start < 0 {
		t.Fatalf("declaration %q not found in source", decl)
	}
	rest := src[start+len(decl):]
	if end := strings.Index(rest, "\nfunc "); end >= 0 {
		return rest[:end]
	}
	return rest
}
