package globalssh

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// isolation_contract_test.go is the executable proof of the isolated-config
// baseline claim's USER-CONFIG half (06-REVIEWS.md's HIGH finding pinned by a
// test rather than assumed): a distinctive value planted in a hermetic SSH
// configuration must be observed by a probe that reads it and MUST NOT be
// observed by the isolated `ssh -G -F /dev/null` probe that replaces the
// per-user file and ignores the system-wide one. The provenance label that
// renders a baseline value is scoped to exactly what this establishes — "the
// value OpenSSH resolves without a user configuration on this machine", never
// "compiled defaults". The system-file half of the claim is documented via
// ssh(1)'s own statement, not re-established here.
//
// Verified platform fact (recorded in 06-01-SUMMARY.md): ssh resolves the
// per-user config from the passwd entry (getpwuid), NOT the $HOME environment
// variable — seeding $HOME alone is invisible to `ssh -G`. The hermetic proof
// therefore feeds the planted file to the real RunSSHG via `-F <file>`, the
// same -F pattern tester.ResolvedVia already uses; `-F /dev/null` is the
// isolated half. This is still the REAL `ssh` binary and the REAL
// BuildProbeDeps seam — no ssh invocation is faked.
func TestIsolatedConfigContract(t *testing.T) {
	if _, err := exec.LookPath("ssh"); err != nil {
		t.Skipf("no ssh binary in PATH (%v) — skipping the hermetic isolated-config contract proof", err)
	}

	dir := t.TempDir()
	configPath := filepath.Join(dir, "config")
	// A distinctive value the isolated `-F /dev/null` configuration can never
	// produce — the compiled baseline resolves hashknownhosts to "no".
	const planted = "HashKnownHosts yes"
	if err := os.WriteFile(configPath, []byte(planted+"\n"), 0o600); err != nil {
		t.Fatalf("planting the distinctive config value: %v", err)
	}

	deps := BuildProbeDeps(configPath)

	plain, err := runProbe(deps, "-G", "-F", configPath, ProbeHost)
	if err != nil {
		t.Fatalf("plain probe (reading the planted file) failed (is the local ssh client healthy?): %v", err)
	}
	isolated, err := runProbe(deps, "-G", "-F", "/dev/null", ProbeHost)
	if err != nil {
		t.Fatalf("isolated `ssh -G -F /dev/null` probe failed: %v", err)
	}

	if got := plain["hashknownhosts"]; got != "yes" {
		t.Errorf("the probe reading the planted file observed hashknownhosts = %q, want the planted %q", got, planted)
	}
	if got := isolated["hashknownhosts"]; got == "yes" {
		t.Errorf("the isolated `-F /dev/null` probe observed the planted value %q — the per-user config is NOT isolated from it", planted)
	}

	// Record what the machine observed so the plan summary can cite it
	// verbatim (the <output> contract requires it).
	t.Logf("observed on this machine: with-planted-file=%q isolated=%q", plain["hashknownhosts"], isolated["hashknownhosts"])
}
