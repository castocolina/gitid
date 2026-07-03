package platform

import (
	"reflect"
	"testing"
)

func TestKeyTypeMapping(t *testing.T) {
	t.Run("AlgorithmForToken maps sk- FIDO2 tokens by exact match, not substring", func(t *testing.T) {
		tests := []struct {
			token string
			want  string
		}{
			// Pitfall 2: the real `ssh -Q key` FIDO2 tokens carry an `sk-` prefix
			// and an `@openssh.com` vendor suffix — never the "ed25519-sk"
			// shorthand used in the requirements docs.
			{"sk-ssh-ed25519@openssh.com", "ed25519-sk"},
			{"ssh-ed25519", "ed25519"},
			{"ssh-rsa", "rsa"},
			{"ecdsa-sha2-nistp256", "ecdsa-p256"},
			{"sk-ecdsa-sha2-nistp256@openssh.com", "ecdsa-sk"},
			// Unknown tokens map to empty — a manipulated PATH `ssh` cannot
			// inject a false catalog entry into downstream logic (T-01-02).
			{"ssh-dss", ""},
			{"ed25519-sk", ""}, // the WRONG shorthand must NOT match via substring
		}
		for _, tt := range tests {
			if got := AlgorithmForToken(tt.token); got != tt.want {
				t.Errorf("AlgorithmForToken(%q) = %q, want %q", tt.token, got, tt.want)
			}
		}
	})

	t.Run("SupportedAlgorithms resolves the catalog names present in a token slice", func(t *testing.T) {
		tokens := parseKeyTypes(sshQKeyFixture)
		got := SupportedAlgorithms(tokens)

		gotSet := make(map[string]bool, len(got))
		for _, a := range got {
			gotSet[a] = true
		}
		want := map[string]bool{
			"ed25519":    true,
			"ed25519-sk": true,
			"ecdsa-p256": true,
			"rsa":        true,
		}
		if !reflect.DeepEqual(gotSet, want) {
			t.Errorf("SupportedAlgorithms(%v) = %v, want set %v", tokens, got, want)
		}
	})
}
