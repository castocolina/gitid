package platform

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

func TestCapabilities(t *testing.T) {
	t.Run("Probe assembles Capabilities from injected deps", func(t *testing.T) {
		deps := Deps{
			ProbeAgent:    func(_ context.Context) AgentStatus { return AgentRunning },
			ProbeFIDO:     func(_ context.Context) FIDOStatus { return FIDOUsable },
			ProbeKeychain: func() KeychainStatus { return KeychainSupported },
			ProbeSSHVersion: func() (SSHVersion, error) {
				return SSHVersion{
					OpenSSHVersion: "9.7p1",
					SSLFlavor:      "LibreSSL",
					SSLVersion:     "3.3.6",
					Raw:            "OpenSSH_9.7p1, LibreSSL 3.3.6",
				}, nil
			},
			ProbeKeyTypes: func() ([]string, error) {
				return []string{"ssh-ed25519", "ssh-rsa"}, nil
			},
		}

		got, err := Probe(context.Background(), deps)
		if err != nil {
			t.Fatalf("Probe() unexpected error: %v", err)
		}
		if got.Agent != AgentRunning {
			t.Errorf("Probe().Agent = %v, want AgentRunning", got.Agent)
		}
		if got.FIDO != FIDOUsable {
			t.Errorf("Probe().FIDO = %v, want FIDOUsable", got.FIDO)
		}
		if got.Keychain != KeychainSupported {
			t.Errorf("Probe().Keychain = %v, want KeychainSupported", got.Keychain)
		}
		if got.SSHVersion.OpenSSHVersion != "9.7p1" {
			t.Errorf("Probe().SSHVersion = %+v, want OpenSSHVersion 9.7p1", got.SSHVersion)
		}
		wantAlgos := []string{"ed25519", "rsa"}
		if !reflect.DeepEqual(got.Algorithms, wantAlgos) {
			t.Errorf("Probe().Algorithms = %v, want %v", got.Algorithms, wantAlgos)
		}
	})

	t.Run("absent FIDO yields FIDOAbsent and a nil error (non-fatal)", func(t *testing.T) {
		deps := Deps{
			ProbeAgent:      func(_ context.Context) AgentStatus { return AgentAbsent },
			ProbeFIDO:       func(_ context.Context) FIDOStatus { return FIDOAbsent },
			ProbeKeychain:   func() KeychainStatus { return KeychainUnsupported },
			ProbeSSHVersion: func() (SSHVersion, error) { return SSHVersion{Raw: "OpenSSH_9.6p1, OpenSSL 3.0.13"}, nil },
			ProbeKeyTypes:   func() ([]string, error) { return nil, nil },
		}

		got, err := Probe(context.Background(), deps)
		if err != nil {
			t.Fatalf("Probe() with absent FIDO/hardware support returned an error (must be non-fatal): %v", err)
		}
		if got.FIDO != FIDOAbsent {
			t.Errorf("Probe().FIDO = %v, want FIDOAbsent", got.FIDO)
		}
	})

	t.Run("FIDOStatus.Usable collapses the three-valued status to a bool", func(t *testing.T) {
		if !FIDOUsable.Usable() {
			t.Error("FIDOUsable.Usable() = false, want true")
		}
		if FIDOAbsent.Usable() {
			t.Error("FIDOAbsent.Usable() = true, want false")
		}
		if FIDOTokenListedOnly.Usable() {
			t.Error("FIDOTokenListedOnly.Usable() = true, want false")
		}
	})

	t.Run("KeychainStatus is supported only on darwin, regardless of agent state", func(t *testing.T) {
		if probeKeychain("linux") != KeychainUnsupported {
			t.Error("probeKeychain(linux) != KeychainUnsupported")
		}
		if probeKeychain("darwin") != KeychainSupported {
			t.Error("probeKeychain(darwin) != KeychainSupported")
		}
		if probeKeychain("windows") != KeychainUnsupported {
			t.Error("probeKeychain(windows) != KeychainUnsupported")
		}
	})

	t.Run("AgentStatus distinguishes absent from a running agent with no keys", func(t *testing.T) {
		t.Setenv("SSH_AUTH_SOCK", "")
		if got := probeAgent(context.Background()); got != AgentAbsent {
			t.Errorf("probeAgent() with no SSH_AUTH_SOCK = %v, want AgentAbsent", got)
		}
	})
}

// TestProbeDepsWiring closes the project's documented "injected-seam wiring
// blindspot": it exercises the REAL BuildProbeDeps() constructor (not just a
// test fake) end to end.
func TestProbeDepsWiring(t *testing.T) {
	deps := BuildProbeDeps()
	if deps.ProbeAgent == nil {
		t.Error("BuildProbeDeps().ProbeAgent is nil")
	}
	if deps.ProbeFIDO == nil {
		t.Error("BuildProbeDeps().ProbeFIDO is nil")
	}
	if deps.ProbeKeychain == nil {
		t.Error("BuildProbeDeps().ProbeKeychain is nil")
	}
	if deps.ProbeSSHVersion == nil {
		t.Error("BuildProbeDeps().ProbeSSHVersion is nil")
	}
	if deps.ProbeKeyTypes == nil {
		t.Error("BuildProbeDeps().ProbeKeyTypes is nil")
	}

	got, err := Probe(context.Background(), deps)
	if err != nil {
		t.Fatalf("Probe(BuildProbeDeps()) unexpected error: %v", err)
	}
	if got.SSHVersion.Raw == "" {
		t.Error("Probe(BuildProbeDeps()) returned an empty SSHVersion.Raw")
	}
}

// TestProbeTimeout proves a hung external probe (a stuck ssh-agent) never
// blocks gitid: probeAgent must return AgentLockedOrUnavailable promptly,
// bounded by probeTimeout, rather than waiting on the fake `ssh-add` that
// never exits (T-01-03).
func TestProbeTimeout(t *testing.T) {
	dir := t.TempDir()
	script := filepath.Join(dir, "ssh-add")
	// #nosec G306 -- test fixture in a t.TempDir(), not a managed gitid file
	if err := os.WriteFile(script, []byte("#!/bin/sh\nsleep 30\n"), 0o755); err != nil {
		t.Fatalf("writing fake hung ssh-add: %v", err)
	}

	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("SSH_AUTH_SOCK", filepath.Join(dir, "agent.sock"))

	oldTimeout := probeTimeout
	probeTimeout = 100 * time.Millisecond
	t.Cleanup(func() { probeTimeout = oldTimeout })

	start := time.Now()
	status := probeAgent(context.Background())
	elapsed := time.Since(start)

	if status != AgentLockedOrUnavailable {
		t.Errorf("probeAgent() with a hung ssh-add = %v, want AgentLockedOrUnavailable", status)
	}
	if elapsed > 2*time.Second {
		t.Errorf("probeAgent() with a hung ssh-add took %v, want it bounded by probeTimeout (did not honor exec.CommandContext timeout)", elapsed)
	}
}
