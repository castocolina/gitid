package keygen

import "strings"

// AlgoInfo describes one of the top-5 SSH key algorithms gitid's create flow
// offers (KEY-01): what it is, whether gitid can generate it today, and
// per-OS availability/variant metadata.
//
// Implemented and Available are deliberately ORTHOGONAL facts:
//   - Implemented reflects whether gitid's registry (registry.go) has a real
//     generator for this algorithm — a static, build-time fact.
//   - Available reflects whether the LOCAL machine's OpenSSH toolchain (and,
//     for -sk entries, libfido2/a connected authenticator) currently
//     supports it — a runtime fact resolved by ResolveAvailability.
//
// A registered-but-stubbed algorithm can be Available (its token is listed
// by `ssh -Q key`) while still Implemented==false; Generatable requires
// BOTH, so gitid never offers it as if it could generate real key material
// (T-01-21).
//
// Ordering and final copy (Security/DarwinNote/LinuxNote wording) are
// deliberately placeholder-but-accurate here; the final top-5 presentation
// order and marketing copy are deferred to Phase 2 design (D-06,
// REQUIREMENTS "Still Open").
type AlgoInfo struct {
	// Name is the algorithm name gitid uses internally — the same string
	// passed as Params.Algo and looked up in the registry.
	Name string
	// QueryToken is the exact string `ssh -Q key` emits for this algorithm
	// (RESEARCH Pitfall 2 — the -sk tokens carry a "sk-" prefix and an
	// "@openssh.com" vendor-extension suffix, NOT the human-friendly name).
	QueryToken string
	// Default marks the one entry (ed25519) the create flow recommends and
	// pre-selects.
	Default bool
	// Implemented reports whether gitid's registry has a real generator for
	// this algorithm (see registry.go). Only ed25519 and rsa-4096 are true
	// today.
	Implemented bool
	// Security is a brief, factual note on the algorithm's security
	// properties (not marketing copy — see Phase 2 design deferral above).
	Security string
	// DarwinNote describes availability/variant considerations on macOS.
	DarwinNote string
	// LinuxNote describes availability/variant considerations on Linux.
	LinuxNote string
	// Available reports whether the LOCAL machine's toolchain currently
	// supports this algorithm. Zero-value (false) until ResolveAvailability
	// is called; Catalog() alone never sets it.
	Available bool
}

// Generatable reports whether a can actually be generated right now: it
// requires BOTH Implemented (gitid has a generator) AND Available (the local
// toolchain supports it). Registry presence alone — or local toolchain
// support alone — never implies generation support; callers MUST use
// Generatable (not Implemented or Available individually) to decide whether
// to offer an algorithm in the create flow (Codex LOW, T-01-21).
func Generatable(a AlgoInfo) bool {
	return a.Implemented && a.Available
}

// isHardwareBacked reports whether a is one of the two FIDO2/hardware-key
// ("-sk") variants, which additionally require a connected, usable
// authenticator to be Available.
func isHardwareBacked(a AlgoInfo) bool {
	return strings.HasSuffix(a.Name, "-sk")
}

// Catalog returns the fixed top-5 algorithm list (KEY-01): ed25519 (the
// default), ed25519-sk, rsa-4096, ecdsa-p256, ecdsa-sk. Every entry's
// Available field is the zero value (false); call ResolveAvailability to
// cross-reference against a specific machine's probe results.
func Catalog() []AlgoInfo {
	return []AlgoInfo{
		{ //nolint:gosec // G101 false positive: public algorithm identifiers below, not credentials
			Name:        "ed25519",
			QueryToken:  "ssh-ed25519",
			Default:     true,
			Implemented: true,
			Security:    "Modern EdDSA signature scheme; small keys, fast, no known practical weaknesses. gitid's recommended default.",
			DarwinNote:  "Supported by the system OpenSSH client on all currently supported macOS releases; no extra install needed.",
			LinuxNote:   "Supported by OpenSSH 6.5+ (2014), present in every current distribution's default OpenSSH package.",
		},
		{
			Name:        "rsa-4096",
			QueryToken:  "ssh-rsa",
			Implemented: true,
			Security:    "Widely compatible legacy algorithm; 4096-bit keys are the minimum gitid generates for adequate long-term margin. Larger keys and slower operations than ed25519.",
			DarwinNote:  "Supported by the system OpenSSH client on all currently supported macOS releases; no extra install needed.",
			LinuxNote:   "Supported by every OpenSSH release in current distributions; no extra install needed.",
		},
		{ //nolint:gosec // G101 false positive: public algorithm identifiers below, not credentials
			Name:        "ecdsa-p256",
			QueryToken:  "ecdsa-sha2-nistp256",
			Implemented: false,
			Security:    "NIST P-256 elliptic-curve signatures; smaller and faster than RSA. gitid does not yet generate this algorithm (registered stub, D-05/D-06).",
			DarwinNote:  "Query token is typically listed by the system OpenSSH client; generation is not yet implemented by gitid regardless of local availability.",
			LinuxNote:   "Query token is typically listed by current distribution OpenSSH packages; generation is not yet implemented by gitid regardless of local availability.",
		},
		{ //nolint:gosec // G101 false positive: public algorithm identifiers below, not credentials
			Name:        "ed25519-sk",
			QueryToken:  "sk-ssh-ed25519@openssh.com",
			Implemented: false,
			Security:    "Hardware-security-key-backed ed25519 (FIDO2/U2F resident or non-resident credential). Requires a physical authenticator for every signing operation. gitid does not yet generate this algorithm (registered stub, D-05/D-06).",
			DarwinNote:  "Requires libfido2 (`brew install libfido2`) and a connected FIDO2 authenticator for the token to be usable, even once implemented.",
			LinuxNote:   "Requires libfido2 (e.g. `apt install libfido2-1`) and a connected FIDO2 authenticator for the token to be usable, even once implemented.",
		},
		{ //nolint:gosec // G101 false positive: public algorithm identifiers below, not credentials
			Name:        "ecdsa-sk",
			QueryToken:  "sk-ecdsa-sha2-nistp256@openssh.com",
			Implemented: false,
			Security:    "Hardware-security-key-backed ECDSA P-256 (FIDO2/U2F resident or non-resident credential). Requires a physical authenticator for every signing operation. gitid does not yet generate this algorithm (registered stub, D-05/D-06).",
			DarwinNote:  "Requires libfido2 (`brew install libfido2`) and a connected FIDO2 authenticator for the token to be usable, even once implemented.",
			LinuxNote:   "Requires libfido2 (e.g. `apt install libfido2-1`) and a connected FIDO2 authenticator for the token to be usable, even once implemented.",
		},
	}
}

// ResolveAvailability returns a copy of cat with Available set by
// cross-referencing each entry's QueryToken against supportedTokens (as
// produced by e.g. internal/platform.ProbeKeyTypes). For the two -sk
// entries, Available additionally requires fidoUsable — a listed sk- token
// alone does not mean the local machine can actually use a hardware key (no
// libfido2, or no connected authenticator).
//
// cat is taken (not mutated) and supportedTokens/fidoUsable are plain data —
// this function deliberately does NOT import internal/platform, so keygen
// stays decoupled from the platform probe package (the debug command in
// 01-06 passes caps.FIDO.Usable() for fidoUsable).
func ResolveAvailability(cat []AlgoInfo, supportedTokens []string, fidoUsable bool) []AlgoInfo {
	present := make(map[string]bool, len(supportedTokens))
	for _, tok := range supportedTokens {
		present[tok] = true
	}

	resolved := make([]AlgoInfo, len(cat))
	for i, a := range cat {
		a.Available = present[a.QueryToken]
		if isHardwareBacked(a) {
			a.Available = a.Available && fidoUsable
		}
		resolved[i] = a
	}
	return resolved
}
