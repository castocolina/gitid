//go:build smoke

package main

// smoke_network_test.go is `make smoke-network-test`'s entry point (D-23):
// a REAL-network, skippable connectivity smoke check — LOCAL/UAT
// convenience only, never a required CI gate (CI stays fully deterministic
// via the FakeSSHDir PTY e2e suite, plan 03-06 Task 1). This build tag
// ("smoke") is deliberately separate from "e2e"/"screenshot" so it never
// runs as a side effect of `make test`/`make test-e2e`/`make
// gate-visual-regression`.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/castocolina/gitid/internal/keygen"
	"github.com/castocolina/gitid/internal/tester"
)

// smokeUnreachableSubstrings are output substrings that mean the NETWORK
// itself (or the provider) was unreachable, distinct from a genuine
// PASS/ReachableNotUploaded/Failure classification — these auto-skip the
// smoke test rather than failing it (D-23's own explicit requirement).
var smokeUnreachableSubstrings = []string{
	"could not resolve",
	"network is unreachable",
	"no route to host",
	"connection timed out",
	"operation timed out",
	"connection refused",
	"temporary failure in name resolution",
}

// TestSmokeNetworkConnectivity runs a REAL `ssh -T` connectivity probe
// against github.com's real alt-SSH endpoint (ssh.github.com:443 — the same
// recipe-canonical pairing the create-flow wizard's default form already
// uses) with a freshly generated, never-registered throwaway key, and
// asserts the outcome is PASS or ReachableNotUploaded. It auto-skips
// (t.Skip, not t.Fail) when the failure looks like the network/provider
// itself was unreachable.
func TestSmokeNetworkConnectivity(t *testing.T) {
	dir := t.TempDir()
	keyPath := filepath.Join(dir, "id_ed25519_smoke")
	mat, err := keygen.GenerateMaterial(keygen.Params{Algo: "ed25519", Identity: "smoke", Comment: "smoke@gitid"})
	if err != nil {
		t.Fatalf("smoke-network-test: generating the throwaway key: %v", err)
	}
	if err := os.WriteFile(keyPath, mat.PrivPEM, 0o600); err != nil {
		t.Fatalf("smoke-network-test: writing the throwaway key: %v", err)
	}

	// knownHostsPath is scoped to this test's TempDir so the smoke probe never
	// touches the real ~/.ssh/known_hosts (D-23's read-only-except-scratch
	// contract) -- StrictHostKeyChecking=accept-new populates it on first use.
	knownHostsPath := filepath.Join(dir, "known_hosts_smoke")
	res := tester.PreWrite(keyPath, "ssh.github.com", 443, knownHostsPath)
	switch res.Outcome {
	case tester.PASS, tester.ReachableNotUploaded:
		t.Logf("smoke-network-test: outcome=%v output=%q", res.Outcome, res.Output)
		return
	default:
		out := strings.ToLower(res.Output)
		for _, needle := range smokeUnreachableSubstrings {
			if strings.Contains(out, needle) {
				t.Skipf("smoke-network-test: skipped — provider/network unreachable: %s", res.Output)
			}
		}
		t.Fatalf("smoke-network-test: unexpected hard Failure against ssh.github.com:443: %s", res.Output)
	}
}
