package globalssh

import (
	"strings"
	"testing"

	"github.com/castocolina/gitid/internal/platform"
)

func TestVersionGate(t *testing.T) {
	p, ok := PolicyFor("StrictHostKeyChecking")
	if !ok {
		t.Fatal("StrictHostKeyChecking policy missing")
	}
	cases := []struct {
		name    string
		version string
		want    VersionOutcome
	}{
		{"below", "7.5", VersionTooOld},
		{"equal", "7.6", VersionAvailable},
		{"above", "9.7p1", VersionAvailable},
		{"two-digit minor beats one-digit", "7.10", VersionAvailable},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, note := VersionGate(platform.SSHVersion{OpenSSHVersion: tc.version}, p)
			if got != tc.want {
				t.Fatalf("VersionGate(%q) = %v, want %v", tc.version, got, tc.want)
			}
			if note == "" {
				t.Fatal("version note must be non-empty when a minimum is set")
			}
		})
	}
}

func TestVersionGateUnverified(t *testing.T) {
	p, ok := PolicyFor("StrictHostKeyChecking")
	if !ok {
		t.Fatal("StrictHostKeyChecking policy missing")
	}
	got, note := VersionGate(platform.SSHVersion{}, p)
	if got != VersionUnverified {
		t.Fatalf("empty version = %v, want VersionUnverified", got)
	}
	if note == "" || !strings.Contains(note, "ssh -V") {
		t.Fatalf("unverified note = %q, want it to name ssh -V", note)
	}
	if strings.Contains(note, VersionNotePrefix) {
		t.Fatalf("unverified note must not use the dynamic prefix; got %q", note)
	}
}

// TestVersionGateNamesTheGatedOptionNotAHardcodedLiteral is the WR-09
// regression: VersionGate is generic over OptionPolicy, so the rendered note
// must name THAT policy's key/recommended value — never a hardcoded
// "accept-new" literal that would describe the wrong option the moment a
// second policy row gains a MinOpenSSH.
func TestVersionGateNamesTheGatedOptionNotAHardcodedLiteral(t *testing.T) {
	synthetic := OptionPolicy{
		Key:         "SendEnv",
		Recommended: "LANG",
		MinOpenSSH:  "8.0",
	}

	tooOldOutcome, tooOldNote := VersionGate(platform.SSHVersion{OpenSSHVersion: "7.9"}, synthetic)
	if tooOldOutcome != VersionTooOld {
		t.Fatalf("VersionGate(7.9) outcome = %v, want VersionTooOld", tooOldOutcome)
	}
	if !strings.Contains(tooOldNote, "SendEnv LANG") {
		t.Errorf("too-old note = %q, want it to name %q, not a hardcoded accept-new literal", tooOldNote, "SendEnv LANG")
	}
	if strings.Contains(tooOldNote, "accept-new") {
		t.Errorf("too-old note = %q, WR-09 regressed: hardcoded accept-new literal reappeared for a different policy", tooOldNote)
	}

	availableOutcome, availableNote := VersionGate(platform.SSHVersion{OpenSSHVersion: "9.0"}, synthetic)
	if availableOutcome != VersionAvailable {
		t.Fatalf("VersionGate(9.0) outcome = %v, want VersionAvailable", availableOutcome)
	}
	if !strings.Contains(availableNote, "SendEnv LANG") {
		t.Errorf("available note = %q, want it to name %q, not a hardcoded accept-new literal", availableNote, "SendEnv LANG")
	}
	if strings.Contains(availableNote, "accept-new") {
		t.Errorf("available note = %q, WR-09 regressed: hardcoded accept-new literal reappeared for a different policy", availableNote)
	}
}

func TestVersionGateNoMinimum(t *testing.T) {
	p, ok := PolicyFor("ForwardAgent")
	if !ok {
		t.Fatal("ForwardAgent policy missing")
	}
	got, note := VersionGate(platform.SSHVersion{OpenSSHVersion: "7.0"}, p)
	if got != VersionAvailable || note != "" {
		t.Fatalf("no-minimum VersionGate() = (%v, %q), want available with empty note", got, note)
	}
}
