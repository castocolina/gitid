package sshconfig

import (
	"strings"
	"testing"

	ssh_config "github.com/kevinburke/ssh_config"
)

// TestProviderMarker_RoundTripStable proves Assumption A4: a # gitid: provider=<p>
// comment placed as the last line of a Host block body survives a
// parse → render (cfg.String()) → parse cycle with the Host alias,
// Hostname, Port, and IdentityFile values byte-identical across both decodes.
//
// This test MUST PASS before Plan 02 builds on the provider-marker design.
// If it fails, the marker placement strategy must be reconsidered (Pitfall 2 /
// RESEARCH.md §Pitfall 2 — SSH round-trip: # gitid: provider= comment position).
func TestProviderMarker_RoundTripStable(t *testing.T) {
	// Build a Host block body with the provider marker as the LAST line
	// (after IdentitiesOnly yes, before the end sentinel).
	// This placement keeps the Host stanza clean and parseable (Pitfall 2).
	const body = `Host testid.github
  Hostname ssh.github.com
  Port 443
  User git
  IdentityFile /home/test/.ssh/id_ed25519_testid
  IdentitiesOnly yes
# gitid: provider=github
`

	// First decode.
	cfg1, err := ssh_config.Decode(strings.NewReader(body))
	if err != nil {
		t.Fatalf("first Decode failed: %v", err)
	}

	// Re-serialize.
	serialized := cfg1.String()

	// The provider comment must survive serialization.
	if !strings.Contains(serialized, "# gitid: provider=github") {
		t.Fatalf("provider marker comment dropped after cfg.String();\ngot:\n%s", serialized)
	}

	// Second decode — must parse the same values as the first.
	cfg2, err := ssh_config.Decode(strings.NewReader(serialized))
	if err != nil {
		t.Fatalf("second Decode (after String()) failed: %v", err)
	}

	// Extract values from both decodes and assert byte-identity.
	const alias = "testid.github"

	hostname1, _ := cfg1.Get(alias, "Hostname")
	hostname2, _ := cfg2.Get(alias, "Hostname")
	if hostname1 != hostname2 {
		t.Errorf("Hostname mismatch across decodes: %q vs %q", hostname1, hostname2)
	}
	if hostname1 != "ssh.github.com" {
		t.Errorf("Hostname wrong: got %q, want %q", hostname1, "ssh.github.com")
	}

	port1, _ := cfg1.Get(alias, "Port")
	port2, _ := cfg2.Get(alias, "Port")
	if port1 != port2 {
		t.Errorf("Port mismatch across decodes: %q vs %q", port1, port2)
	}
	if port1 != "443" {
		t.Errorf("Port wrong: got %q, want %q", port1, "443")
	}

	idFile1, _ := cfg1.Get(alias, "IdentityFile")
	idFile2, _ := cfg2.Get(alias, "IdentityFile")
	if idFile1 != idFile2 {
		t.Errorf("IdentityFile mismatch across decodes: %q vs %q", idFile1, idFile2)
	}
	if idFile1 != "/home/test/.ssh/id_ed25519_testid" {
		t.Errorf("IdentityFile wrong: got %q, want %q", idFile1, "/home/test/.ssh/id_ed25519_testid")
	}

	// Also confirm the alias itself is present in both parses.
	found1 := false
	found2 := false
	for _, host := range cfg1.Hosts {
		for _, p := range host.Patterns {
			if p.String() == alias {
				found1 = true
			}
		}
	}
	for _, host := range cfg2.Hosts {
		for _, p := range host.Patterns {
			if p.String() == alias {
				found2 = true
			}
		}
	}
	if !found1 {
		t.Errorf("alias %q not found in first decode", alias)
	}
	if !found2 {
		t.Errorf("alias %q not found in second decode (after String())", alias)
	}
}
