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
