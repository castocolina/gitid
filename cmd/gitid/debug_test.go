package main

import (
	"bytes"
	"context"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/castocolina/gitid/internal/gitconfig"
	"github.com/castocolina/gitid/internal/identity"
	"github.com/castocolina/gitid/internal/keygen"
	"github.com/castocolina/gitid/internal/platform"
)

// TestDebugCommand_Registered asserts `gitid debug` is wired on the root
// command tree (main.go registration).
func TestDebugCommand_Registered(t *testing.T) {
	root := newRootCmd()
	cmd, _, err := root.Find([]string{"debug", "caps"})
	if err != nil {
		t.Fatalf("expected 'debug caps' to be a registered command: %v", err)
	}
	if cmd.Use != "caps" {
		t.Errorf("expected the resolved command's Use to be 'caps', got %q", cmd.Use)
	}
}

// fakeCapsDeps returns a platform.Deps fully wired to deterministic fakes
// (no real exec calls), so TestDebug* runs hermetically and fast.
func fakeCapsDeps() platform.Deps {
	return platform.Deps{
		ProbeAgent:    func(context.Context) platform.AgentStatus { return platform.AgentRunning },
		ProbeFIDO:     func(context.Context) platform.FIDOStatus { return platform.FIDOAbsent },
		ProbeKeychain: func() platform.KeychainStatus { return platform.KeychainUnsupported },
		ProbeSSHVersion: func() (platform.SSHVersion, error) {
			return platform.SSHVersion{
				OpenSSHVersion: "9.7p1",
				SSLFlavor:      "LibreSSL",
				SSLVersion:     "3.3.6",
				Raw:            "OpenSSH_9.7p1, LibreSSL 3.3.6",
			}, nil
		},
		ProbeKeyTypes: func() ([]string, error) {
			return []string{"ssh-ed25519", "ssh-rsa", "ecdsa-sha2-nistp256"}, nil
		},
	}
}

// fakeInventoryDeps returns an identity.InventoryDeps wired to seed one
// complete identity, so TestDebug* exercises the printInventory path without
// touching the real filesystem.
func fakeInventoryDeps() identity.InventoryDeps {
	const sshConfig = "# BEGIN gitid managed: work\n" +
		"Host work.github.com\n" +
		"  HostName ssh.github.com\n" +
		"  Port 443\n" +
		"  User git\n" +
		"  IdentityFile ~/.ssh/id_ed25519_work\n" +
		"  IdentitiesOnly yes\n" +
		"# END gitid managed: work\n"
	const gitconfigContent = "# BEGIN gitid managed: work\n" +
		"[includeIf \"gitdir:~/git/work/\"]\n" +
		"\tpath = ~/.gitconfig.d/work\n" +
		"# END gitid managed: work\n"

	return identity.InventoryDeps{
		ReadSSHConfig: func() ([]byte, error) { return []byte(sshConfig), nil },
		ReadGitconfig: func() ([]byte, error) { return []byte(gitconfigContent), nil },
		ReadFragment: func(string) (gitconfig.FragmentInfo, error) {
			return gitconfig.FragmentInfo{Missing: true}, nil
		},
		Stat:         func(string) (os.FileInfo, error) { return nil, os.ErrNotExist },
		ListKeyFiles: func() ([]string, error) { return nil, nil },
	}
}

// TestDebugCaps_PrintsAllThreeSections asserts the command's output
// contains a capabilities section, a catalog section, and a per-identity
// state section, using fully-fake deps (hermetic, no real exec/filesystem).
func TestDebugCaps_PrintsAllThreeSections(t *testing.T) {
	var out bytes.Buffer

	caps, err := platform.Probe(context.Background(), fakeCapsDeps())
	if err != nil {
		t.Fatalf("platform.Probe: %v", err)
	}
	printCapabilities(&out, caps)

	cat := keygen.ResolveAvailability(keygen.Catalog(), caps.KeyTypes, caps.FIDO.Usable())
	printCatalog(&out, cat)

	inv, err := identity.BuildInventory(fakeInventoryDeps())
	if err != nil {
		t.Fatalf("identity.BuildInventory: %v", err)
	}
	printInventory(&out, inv)

	got := out.String()

	wantSections := []string{
		"=== Capabilities ===",
		"=== Algorithm Catalog ===",
		"=== Identities ===",
	}
	for _, want := range wantSections {
		if !strings.Contains(got, want) {
			t.Errorf("expected output to contain section %q; got:\n%s", want, got)
		}
	}

	// KEY-01: ed25519's token (ssh-ed25519) is in the fake ProbeKeyTypes list,
	// so it must resolve Available=true, proving the raw-token wiring
	// (caps.KeyTypes, not caps.Algorithms).
	if !strings.Contains(got, "ed25519 (default)") {
		t.Errorf("expected catalog to list 'ed25519 (default)'; got:\n%s", got)
	}
	if !strings.Contains(got, "available: true") {
		t.Errorf("expected at least one catalog entry to resolve available: true from caps.KeyTypes; got:\n%s", got)
	}

	// MGR-02: the seeded "work" identity's IdentityHealth must be printed.
	if !strings.Contains(got, "work") {
		t.Errorf("expected output to contain the seeded identity name 'work'; got:\n%s", got)
	}
}

// TestDebugCaps_NoSecretLeakage asserts the output NEVER contains a PEM
// "PRIVATE KEY" marker, the substrings "passphrase"/"Passphrase"/"PrivPEM",
// or a full-environment dump marker (T-01-15).
func TestDebugCaps_NoSecretLeakage(t *testing.T) {
	var out bytes.Buffer

	caps, err := platform.Probe(context.Background(), fakeCapsDeps())
	if err != nil {
		t.Fatalf("platform.Probe: %v", err)
	}
	printCapabilities(&out, caps)

	cat := keygen.ResolveAvailability(keygen.Catalog(), caps.KeyTypes, caps.FIDO.Usable())
	printCatalog(&out, cat)

	inv, err := identity.BuildInventory(fakeInventoryDeps())
	if err != nil {
		t.Fatalf("identity.BuildInventory: %v", err)
	}
	printInventory(&out, inv)

	got := out.String()

	forbidden := []string{
		"PRIVATE KEY",
		"passphrase",
		"Passphrase",
		"PrivPEM",
	}
	for _, marker := range forbidden {
		if strings.Contains(got, marker) {
			t.Errorf("output must never contain %q; got:\n%s", marker, got)
		}
	}

	// A full-environment dump would contain PATH= (present in every process
	// environment); assert it is absent from the command's output.
	if strings.Contains(got, "PATH=") {
		t.Errorf("output must never contain a full-environment dump (found PATH=); got:\n%s", got)
	}
}

// TestDebugCaps_ProbeError propagates a probe error instead of panicking
// or silently swallowing it (Rule 1/2 correctness — a broken probe seam must
// surface, not print a misleadingly empty report).
func TestDebugCaps_ProbeError(t *testing.T) {
	deps := fakeCapsDeps()
	wantErr := errors.New("boom")
	deps.ProbeSSHVersion = func() (platform.SSHVersion, error) { return platform.SSHVersion{}, wantErr }

	var out bytes.Buffer
	err := runDebugCapsWithDeps(context.Background(), &out, deps, fakeInventoryDeps())
	if err == nil {
		t.Fatal("expected an error when the ssh-version probe fails, got nil")
	}
	if !errors.Is(err, wantErr) {
		t.Errorf("expected wrapped error to be %v, got: %v", wantErr, err)
	}
}
