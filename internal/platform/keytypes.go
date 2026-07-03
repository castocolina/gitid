package platform

// algorithmToken maps every `ssh -Q key` protocol token gitid recognizes to
// its catalog algorithm name. Matching is by exact token, never a substring
// or the informal "-sk" shorthand used in the requirements docs (Pitfall 2):
// the real FIDO2 tokens carry an `sk-` prefix and an `@openssh.com` vendor
// suffix. [VERIFIED: `ssh -Q key` run directly on the research/dev machine
// this session, OpenSSH_9.7p1].
var algorithmToken = map[string]string{
	"ssh-ed25519":                        "ed25519",
	"sk-ssh-ed25519@openssh.com":         "ed25519-sk",
	"ssh-rsa":                            "rsa",
	"ecdsa-sha2-nistp256":                "ecdsa-p256",
	"sk-ecdsa-sha2-nistp256@openssh.com": "ecdsa-sk",
}

// AlgorithmForToken maps a single `ssh -Q key` protocol token to gitid's
// catalog algorithm name. Unknown tokens map to "" (T-01-02: a manipulated
// PATH `ssh` cannot inject a false catalog entry into downstream logic by
// emitting an unrecognized token).
func AlgorithmForToken(token string) string {
	return algorithmToken[token]
}

// SupportedAlgorithms maps a slice of `ssh -Q key` tokens to the set of
// catalog algorithm names present in it, de-duplicated and in the order
// first encountered.
func SupportedAlgorithms(tokens []string) []string {
	seen := make(map[string]bool, len(tokens))
	out := make([]string, 0, len(tokens))
	for _, tok := range tokens {
		name := AlgorithmForToken(tok)
		if name == "" || seen[name] {
			continue
		}
		seen[name] = true
		out = append(out, name)
	}
	return out
}
