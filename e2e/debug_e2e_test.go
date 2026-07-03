//go:build e2e

package e2e

// debug_e2e_test.go: drives the REAL gitid binary's `debug caps` subcommand
// end-to-end (DLV-07), closing the recurring injected-seam wiring blindspot
// for this new command surface (T-01-16). The command is non-interactive
// (prints and exits), so it is driven via a plain exec.Command against the
// harness's sandboxed HOME + built binary — the same pattern as
// adopt_e2e_test.go — rather than the raw-keystroke PTY harness reserved for
// interactive TUI surfaces.
//
// Contract: this test must exercise the REAL platform.BuildProbeDeps +
// identity.BuildInventoryDeps wiring (via the built binary), not fakes, and
// must assert the output NEVER leaks secret material.

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
)

// TestDebugCaps_RealWiring drives `gitid debug caps` against a sandboxed
// HOME seeded with one managed identity ("personal") and asserts the real
// output contains all three sections: the algorithm catalog (with the
// ed25519 entry resolving Available=true from the sandbox's real
// `ssh -Q key` tokens — proving the raw-token wiring, T-01-16), the
// structured capability probe, and the seeded identity's IdentityHealth.
func TestDebugCaps_RealWiring(t *testing.T) {
	home := SandboxHome(t)
	bin := BuildBinary(t)

	seedMinimalIdentity(t, home, "personal")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	var stdout, stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, bin, "debug", "caps") //nolint:gosec // bin from BuildBinary; fixed args
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	cmd.Env = append(os.Environ(), "HOME="+home)

	if err := cmd.Run(); err != nil {
		t.Fatalf("gitid debug caps failed: %v\nstdout: %s\nstderr: %s",
			err, stdout.String(), stderr.String())
	}

	got := stdout.String()

	// The three sections a debug/list command must surface (KEY-01, PLAT-01, MGR-02).
	wantSections := []string{
		"=== Capabilities ===",
		"=== Algorithm Catalog ===",
		"=== Identities ===",
	}
	for _, want := range wantSections {
		if !strings.Contains(got, want) {
			t.Errorf("expected real debug caps output to contain section %q; got:\n%s", want, got)
		}
	}

	// KEY-01 / T-01-16: the ed25519 catalog entry must resolve Available=true
	// from the sandbox's real `ssh -Q key` tokens (caps.KeyTypes), proving the
	// raw-token wiring end-to-end — a manipulated/absent-tokens bug would show
	// Available=false for every entry (the exact regression this test blocks).
	if idx := strings.Index(got, "ed25519 (default)"); idx == -1 {
		t.Errorf("expected the catalog to list 'ed25519 (default)'; got:\n%s", got)
	} else {
		entrySection := got[idx:]
		if end := strings.Index(entrySection[1:], "\n\n"); end != -1 {
			entrySection = entrySection[:end+1]
		}
		if !strings.Contains(entrySection, "available: true") {
			t.Errorf("expected the ed25519 catalog entry to resolve available: true from the real ssh -Q key tokens; entry:\n%s", entrySection)
		}
	}

	// MGR-02: the seeded "personal" identity's IdentityHealth must be printed,
	// consumed from the real identity.BuildInventoryDeps wiring (not a fake).
	if !strings.Contains(got, "personal") {
		t.Errorf("expected output to contain the seeded identity name 'personal'; got:\n%s", got)
	}
	if !strings.Contains(got, "identity state:") || !strings.Contains(got, "key state:") {
		t.Errorf("expected per-identity IdentityHealth (identity state / key state) in output; got:\n%s", got)
	}

	// T-01-15: the output must NEVER leak secret material — no PEM PRIVATE KEY
	// marker, no passphrase field/value, no PrivPEM reference, no full-env dump.
	forbidden := []string{
		"PRIVATE KEY",
		"passphrase",
		"Passphrase",
		"PrivPEM",
	}
	for _, marker := range forbidden {
		if strings.Contains(got, marker) {
			t.Errorf("real debug caps output must never contain %q; got:\n%s", marker, got)
		}
	}
	if strings.Contains(got, "PATH=") {
		t.Errorf("real debug caps output must never contain a full-environment dump (found PATH=); got:\n%s", got)
	}
}
