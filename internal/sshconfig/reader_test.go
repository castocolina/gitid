package sshconfig

import (
	"testing"

	"github.com/castocolina/gitid/internal/filewriter"
)

// TestParseManagedHosts_Empty verifies that empty content returns an empty map
// with no error.
func TestParseManagedHosts_Empty(t *testing.T) {
	got, err := ParseManagedHosts([]byte(""))
	if err != nil {
		t.Fatalf("ParseManagedHosts on empty content returned error: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("expected empty map for empty content, got %d entries", len(got))
	}
}

// TestParseManagedHosts_TwoBlocks verifies that two managed Host blocks are
// parsed into two SSHHostInfo entries with correct fields.
func TestParseManagedHosts_TwoBlocks(t *testing.T) {
	personalBody := "Host personal.github.com\n\tHostname ssh.github.com\n\tPort 443\n\tIdentityFile ~/.ssh/id_ed25519_personal\n\tIdentitiesOnly yes\n"
	workBody := "Host work.github.com\n\tHostname ssh.github.com\n\tPort 22\n\tIdentityFile ~/.ssh/id_ed25519_work\n\tIdentitiesOnly yes\n"

	content := []byte(
		filewriter.BeginPrefix + "personal\n" + personalBody + filewriter.EndPrefix + "personal\n" +
			filewriter.BeginPrefix + "work\n" + workBody + filewriter.EndPrefix + "work\n",
	)

	got, err := ParseManagedHosts(content)
	if err != nil {
		t.Fatalf("ParseManagedHosts returned error: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 entries, got %d: %v", len(got), got)
	}

	personal, ok := got["personal"]
	if !ok {
		t.Fatal("missing 'personal' entry in result")
	}
	if personal.Alias != "personal.github.com" {
		t.Errorf("personal Alias: got %q want %q", personal.Alias, "personal.github.com")
	}
	if personal.Hostname != "ssh.github.com" {
		t.Errorf("personal Hostname: got %q want %q", personal.Hostname, "ssh.github.com")
	}
	if personal.Port != 443 {
		t.Errorf("personal Port: got %d want 443", personal.Port)
	}
	if personal.IdentityFile != "~/.ssh/id_ed25519_personal" {
		t.Errorf("personal IdentityFile: got %q want ~/.ssh/id_ed25519_personal", personal.IdentityFile)
	}
	if !personal.IdentitiesOnly {
		t.Error("personal IdentitiesOnly: expected true")
	}

	work, ok := got["work"]
	if !ok {
		t.Fatal("missing 'work' entry in result")
	}
	if work.Port != 22 {
		t.Errorf("work Port: got %d want 22", work.Port)
	}
}

// TestParseManagedHosts_GlobalSkipped verifies that the _global block is
// skipped and not included in the result map.
func TestParseManagedHosts_GlobalSkipped(t *testing.T) {
	globalBody := "Host *\n\tAddKeysToAgent yes\n"
	workBody := "Host work.github.com\n\tHostname ssh.github.com\n\tPort 22\n\tIdentityFile ~/.ssh/id_ed25519_work\n\tIdentitiesOnly yes\n"

	content := []byte(
		filewriter.BeginPrefix + "work\n" + workBody + filewriter.EndPrefix + "work\n" +
			filewriter.BeginPrefix + "_global\n" + globalBody + filewriter.EndPrefix + "_global\n",
	)

	got, err := ParseManagedHosts(content)
	if err != nil {
		t.Fatalf("ParseManagedHosts returned error: %v", err)
	}
	if _, ok := got["_global"]; ok {
		t.Error("_global block must be skipped, but found in result")
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 entry (only 'work'), got %d: %v", len(got), got)
	}
}

// TestParseManagedHosts_ImplicitHostStar verifies that the implicit Host *
// inserted by the kevinburke parser is skipped (Pitfall A guard).
func TestParseManagedHosts_ImplicitHostStar(t *testing.T) {
	// A block body that starts with a real Host stanza; the parser inserts
	// an implicit Host * as cfg.Hosts[0].
	body := "Host work.github.com\n\tHostname ssh.github.com\n\tPort 22\n\tIdentityFile ~/.ssh/id_ed25519_work\n\tIdentitiesOnly yes\n"
	content := []byte(filewriter.BeginPrefix + "work\n" + body + filewriter.EndPrefix + "work\n")

	got, err := ParseManagedHosts(content)
	if err != nil {
		t.Fatalf("ParseManagedHosts returned error: %v", err)
	}
	// Should return exactly one entry — the real work block.
	if len(got) != 1 {
		t.Fatalf("expected 1 entry (work), got %d", len(got))
	}
	if got["work"].Alias != "work.github.com" {
		t.Errorf("work Alias: got %q want %q", got["work"].Alias, "work.github.com")
	}
}

// TestParseManagedHosts_PortUnsetWhenAbsent verifies that when no Port directive
// is present in the block body, SSHHostInfo.Port is 0 ("unset") rather than a
// fabricated 22 (WR-06). gitid alt-ssh endpoints use 443, so guessing 22 would
// mislead reconstruction/list; the display/use layer applies the real default.
func TestParseManagedHosts_PortUnsetWhenAbsent(t *testing.T) {
	body := "Host work.github.com\n\tHostname ssh.github.com\n\tIdentityFile ~/.ssh/id_ed25519_work\n"
	content := []byte(filewriter.BeginPrefix + "work\n" + body + filewriter.EndPrefix + "work\n")

	got, err := ParseManagedHosts(content)
	if err != nil {
		t.Fatalf("ParseManagedHosts returned error: %v", err)
	}
	if got["work"].Port != 0 {
		t.Errorf("Port: got %d want 0 (unset, not fabricated 22)", got["work"].Port)
	}
}

// TestParseManagedHosts_WithProviderMarker verifies D-11: a managed block
// containing "# gitid: provider=gitlab" returns SSHHostInfo.Provider == "gitlab"
// with all other fields correctly parsed.
func TestParseManagedHosts_WithProviderMarker(t *testing.T) {
	body := "Host work.gitlab.com\n\tHostname altssh.gitlab.com\n\tPort 443\n\tUser git\n\tIdentityFile ~/.ssh/id_ed25519_work\n\tIdentitiesOnly yes\n# gitid: provider=gitlab\n"
	content := []byte(filewriter.BeginPrefix + "work\n" + body + filewriter.EndPrefix + "work\n")

	got, err := ParseManagedHosts(content)
	if err != nil {
		t.Fatalf("ParseManagedHosts returned error: %v", err)
	}
	info, ok := got["work"]
	if !ok {
		t.Fatal("missing 'work' entry in result")
	}
	if info.Provider != "gitlab" {
		t.Errorf("Provider: got %q want %q", info.Provider, "gitlab")
	}
	if info.Alias != "work.gitlab.com" {
		t.Errorf("Alias: got %q want %q", info.Alias, "work.gitlab.com")
	}
	if info.Hostname != "altssh.gitlab.com" {
		t.Errorf("Hostname: got %q want %q", info.Hostname, "altssh.gitlab.com")
	}
	if info.Port != 443 {
		t.Errorf("Port: got %d want 443", info.Port)
	}
	if !info.IdentitiesOnly {
		t.Error("IdentitiesOnly: expected true")
	}
}

// TestParseManagedHosts_MarkerlessBlock verifies D-13: a managed block without
// the provider marker returns SSHHostInfo.Provider == "" (no crash, backward compat).
func TestParseManagedHosts_MarkerlessBlock(t *testing.T) {
	body := "Host work.github.com\n\tHostname ssh.github.com\n\tPort 22\n\tIdentityFile ~/.ssh/id_ed25519_work\n\tIdentitiesOnly yes\n"
	content := []byte(filewriter.BeginPrefix + "work\n" + body + filewriter.EndPrefix + "work\n")

	got, err := ParseManagedHosts(content)
	if err != nil {
		t.Fatalf("ParseManagedHosts returned error: %v", err)
	}
	info, ok := got["work"]
	if !ok {
		t.Fatal("missing 'work' entry in result")
	}
	if info.Provider != "" {
		t.Errorf("Provider: got %q want empty string for markerless block", info.Provider)
	}
}

// TestRenderAndParseManagedHosts_ProviderRoundTrip verifies the full write/read
// path: render a block with provider → wrap in managed sentinels → ParseManagedHosts
// → Provider and all fields match input (extends A4 proof to the full path).
func TestRenderAndParseManagedHosts_ProviderRoundTrip(t *testing.T) {
	body := RenderHostBlock("personal.github.com", "ssh.github.com", 443, "~/.ssh/id_ed25519_personal", "github")
	content := []byte(filewriter.BeginPrefix + "personal\n" + body + filewriter.EndPrefix + "personal\n")

	got, err := ParseManagedHosts(content)
	if err != nil {
		t.Fatalf("ParseManagedHosts returned error: %v", err)
	}
	info, ok := got["personal"]
	if !ok {
		t.Fatal("missing 'personal' entry in result")
	}
	if info.Provider != "github" {
		t.Errorf("Provider round-trip: got %q want %q", info.Provider, "github")
	}
	if info.Alias != "personal.github.com" {
		t.Errorf("Alias round-trip: got %q want %q", info.Alias, "personal.github.com")
	}
	if info.Hostname != "ssh.github.com" {
		t.Errorf("Hostname round-trip: got %q want %q", info.Hostname, "ssh.github.com")
	}
	if info.Port != 443 {
		t.Errorf("Port round-trip: got %d want 443", info.Port)
	}
	if !info.IdentitiesOnly {
		t.Error("IdentitiesOnly round-trip: expected true")
	}
}
