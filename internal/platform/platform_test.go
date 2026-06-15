package platform

import (
	"strings"
	"testing"
)

// sshQKeyFixture is a verified `ssh -Q key` multi-line output (OpenSSH 9.x).
// The parser must extract every token, including the cert/sk variants, so that
// membership tests against the base algorithm names (ssh-ed25519, ssh-rsa,
// ecdsa-sha2-nistp256) work directly.
const sshQKeyFixture = `ssh-ed25519
ssh-ed25519-cert-v01@openssh.com
sk-ssh-ed25519@openssh.com
sk-ssh-ed25519-cert-v01@openssh.com
ecdsa-sha2-nistp256
ecdsa-sha2-nistp256-cert-v01@openssh.com
ecdsa-sha2-nistp384
ssh-dss
ssh-rsa
ssh-rsa-cert-v01@openssh.com
`

func TestParseKeyTypes(t *testing.T) {
	got := parseKeyTypes(sshQKeyFixture)

	want := map[string]bool{
		"ssh-ed25519":         true,
		"ecdsa-sha2-nistp256": true,
		"ssh-rsa":             true,
		"ssh-dss":             true,
	}
	gotSet := map[string]bool{}
	for _, tok := range got {
		gotSet[tok] = true
	}
	for tok := range want {
		if !gotSet[tok] {
			t.Errorf("parseKeyTypes: expected token %q in result, got %v", tok, got)
		}
	}
	// No empty tokens may survive trimming.
	for _, tok := range got {
		if strings.TrimSpace(tok) == "" {
			t.Errorf("parseKeyTypes: produced an empty token in %v", got)
		}
	}
}

func TestSelectAlgorithm(t *testing.T) {
	tests := []struct {
		name       string
		supported  []string
		wantAlgo   string
		wantWarned bool
		wantErr    bool
	}{
		{
			name:       "ed25519 preferred when present",
			supported:  []string{"ssh-ed25519", "ecdsa-sha2-nistp256", "ssh-rsa"},
			wantAlgo:   "ed25519",
			wantWarned: false,
		},
		{
			name:       "rsa fallback when ed25519 absent",
			supported:  []string{"ssh-rsa", "ecdsa-sha2-nistp256"},
			wantAlgo:   "rsa",
			wantWarned: true,
		},
		{
			name:       "ecdsa fallback when only ecdsa present",
			supported:  []string{"ecdsa-sha2-nistp256"},
			wantAlgo:   "ecdsa",
			wantWarned: true,
		},
		{
			name:      "empty support set errors",
			supported: []string{},
			wantErr:   true,
		},
		{
			name:      "unsupported-only support set errors",
			supported: []string{"ssh-dss"},
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			algo, warned, err := SelectAlgorithm(tt.supported)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("SelectAlgorithm(%v): expected error, got nil (algo=%q)", tt.supported, algo)
				}
				// D-14: error must carry actionable per-OS install guidance.
				hint := InstallHint("openssh", CurrentOS())
				if !strings.Contains(err.Error(), strings.Split(hint, "\n")[0]) {
					t.Errorf("SelectAlgorithm error %q does not carry install hint %q", err.Error(), hint)
				}
				return
			}
			if err != nil {
				t.Fatalf("SelectAlgorithm(%v): unexpected error: %v", tt.supported, err)
			}
			if algo != tt.wantAlgo {
				t.Errorf("SelectAlgorithm(%v): algo = %q, want %q", tt.supported, algo, tt.wantAlgo)
			}
			if warned != tt.wantWarned {
				t.Errorf("SelectAlgorithm(%v): warned = %v, want %v", tt.supported, warned, tt.wantWarned)
			}
		})
	}
}

func TestInstallHint(t *testing.T) {
	// OpenSSH tool on darwin — must contain brew and openssh.
	darwin := InstallHint("openssh", "darwin")
	if !strings.Contains(darwin, "brew") {
		t.Errorf("InstallHint(openssh, darwin) = %q, want it to contain %q", darwin, "brew")
	}
	if !strings.Contains(darwin, "openssh") {
		t.Errorf("InstallHint(openssh, darwin) = %q, want it to contain %q", darwin, "openssh")
	}

	// OpenSSH tool on linux — must contain all three package managers.
	linux := InstallHint("openssh", "linux")
	for _, want := range []string{"apt", "dnf", "pacman"} {
		if !strings.Contains(linux, want) {
			t.Errorf("InstallHint(openssh, linux) = %q, want it to contain %q", linux, want)
		}
	}

	// Unknown OS must still return non-empty guidance (the OpenSSH project link).
	other := InstallHint("openssh", "plan9")
	if strings.TrimSpace(other) == "" {
		t.Errorf("InstallHint(openssh, unknown) returned empty guidance")
	}

	// git tool on darwin — must contain brew and git.
	gitDarwin := InstallHint("git", "darwin")
	if !strings.Contains(gitDarwin, "brew") {
		t.Errorf("InstallHint(git, darwin) = %q, want it to contain %q", gitDarwin, "brew")
	}
	if !strings.Contains(gitDarwin, "git") {
		t.Errorf("InstallHint(git, darwin) = %q, want it to contain %q", gitDarwin, "git")
	}

	// git tool on linux — must contain apt, dnf, and pacman lines.
	gitLinux := InstallHint("git", "linux")
	for _, want := range []string{"apt", "dnf", "pacman"} {
		if !strings.Contains(gitLinux, want) {
			t.Errorf("InstallHint(git, linux) = %q, want it to contain %q", gitLinux, want)
		}
	}

	// clipboard tool on linux — must contain xclip (Linux clipboard default).
	clipLinux := InstallHint("clipboard", "linux")
	if !strings.Contains(clipLinux, "xclip") {
		t.Errorf("InstallHint(clipboard, linux) = %q, want it to contain %q", clipLinux, "xclip")
	}

	// clipboard tool on darwin — must contain brew.
	clipDarwin := InstallHint("clipboard", "darwin")
	if !strings.Contains(clipDarwin, "brew") {
		t.Errorf("InstallHint(clipboard, darwin) = %q, want it to contain %q", clipDarwin, "brew")
	}

	// unknown OS shows all four package manager lines.
	unknownOS := InstallHint("git", "unknown-os")
	for _, want := range []string{"brew", "apt", "dnf", "pacman"} {
		if !strings.Contains(unknownOS, want) {
			t.Errorf("InstallHint(git, unknown-os) = %q, want it to contain %q", unknownOS, want)
		}
	}
}

func TestSupportsUseKeychain(t *testing.T) {
	if !SupportsUseKeychain("darwin") {
		t.Error("SupportsUseKeychain(darwin) = false, want true")
	}
	if SupportsUseKeychain("linux") {
		t.Error("SupportsUseKeychain(linux) = true, want false")
	}
}
