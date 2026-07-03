package platform

import "testing"

func TestParseSSHVersion(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want SSHVersion
	}{
		{
			// [VERIFIED: `ssh -V` run directly on the research/dev machine, OpenSSH_9.7p1]
			name: "macOS LibreSSL flavor",
			in:   "OpenSSH_9.7p1, LibreSSL 3.3.6\n",
			want: SSHVersion{
				OpenSSHVersion: "9.7p1",
				SSLFlavor:      "LibreSSL",
				SSLVersion:     "3.3.6",
				Raw:            "OpenSSH_9.7p1, LibreSSL 3.3.6",
			},
		},
		{
			// [CITED: WebSearch, cross-referenced against multiple sources]
			name: "Linux OpenSSL flavor",
			in:   "OpenSSH_9.6p1, OpenSSL 3.0.13\n",
			want: SSHVersion{
				OpenSSHVersion: "9.6p1",
				SSLFlavor:      "OpenSSL",
				SSLVersion:     "3.0.13",
				Raw:            "OpenSSH_9.6p1, OpenSSL 3.0.13",
			},
		},
		{
			// [VERIFIED: `ssh -V` on the ubuntu-latest CI runner this session:
			// Debian/Ubuntu inserts a distro-portable suffix between the version
			// and the comma, which the original regex failed to skip.]
			name: "Debian/Ubuntu distro suffix before the comma",
			in:   "OpenSSH_9.6p1 Ubuntu-3ubuntu13.16, OpenSSL 3.0.13 30 Jan 2024\n",
			want: SSHVersion{
				OpenSSHVersion: "9.6p1",
				SSLFlavor:      "OpenSSL",
				SSLVersion:     "3.0.13",
				Raw:            "OpenSSH_9.6p1 Ubuntu-3ubuntu13.16, OpenSSL 3.0.13 30 Jan 2024",
			},
		},
		{
			name: "malformed input returns zero-value fields with Raw preserved, no panic",
			in:   "not a version string at all",
			want: SSHVersion{Raw: "not a version string at all"},
		},
		{
			name: "empty input returns a zero-value SSHVersion",
			in:   "",
			want: SSHVersion{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseSSHVersion(tt.in)
			if got != tt.want {
				t.Errorf("parseSSHVersion(%q) = %+v, want %+v", tt.in, got, tt.want)
			}
		})
	}
}

// TestProbeSSHVersionReturnsStruct proves ProbeSSHVersion returns a
// structured SSHVersion (not a bare, pre-formatted string) so downstream
// callers (01-06) render fields directly and never re-parse a string.
func TestProbeSSHVersionReturnsStruct(t *testing.T) {
	v, err := ProbeSSHVersion()
	if err != nil {
		t.Fatalf("ProbeSSHVersion() unexpected error: %v", err)
	}
	if v.Raw == "" {
		t.Errorf("ProbeSSHVersion() returned an SSHVersion with an empty Raw field")
	}
	if v.OpenSSHVersion == "" {
		t.Errorf("ProbeSSHVersion() returned an SSHVersion with an empty OpenSSHVersion field (Raw=%q)", v.Raw)
	}
}
