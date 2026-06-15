package platform

import (
	"fmt"
	"os/exec"
	"runtime"
	"strings"
)

// fallbackChain is the deliberate, fixed key-algorithm preference order (D-09).
// gitid never offers a free picker: it always prefers ed25519, then rsa-4096,
// then ecdsa, and stops with install guidance (D-14) when none are available.
//
// Each entry maps the canonical `ssh -Q key` query token to the algorithm name
// gitid uses internally. The slice order encodes the preference: index 0 is the
// default (ed25519), and any selection at index > 0 is a warned downgrade.
type algoCandidate struct {
	queryToken string // membership token as emitted by the `ssh -Q key` query
	name       string // algorithm name gitid uses downstream
}

// The query tokens below are public OpenSSH algorithm identifiers (the exact
// strings `ssh -Q key` emits), not secrets. gosec G101 pattern-matches the
// "ssh-"/"ecdsa-" prefixes as possible hardcoded credentials; that is a false
// positive here, so the assignments are individually annotated.
var fallbackChain = []algoCandidate{
	{queryToken: "ssh-ed25519", name: "ed25519"},       //nolint:gosec // G101 false positive: public algorithm identifier, not a credential
	{queryToken: "ssh-rsa", name: "rsa"},               //nolint:gosec // G101 false positive: public algorithm identifier, not a credential
	{queryToken: "ecdsa-sha2-nistp256", name: "ecdsa"}, //nolint:gosec // G101 false positive: public algorithm identifier, not a credential
}

// CurrentOS returns the running operating system token (e.g. "darwin",
// "linux"). It is a thin seam over runtime.GOOS so callers and tests can reason
// about the platform without importing runtime directly.
func CurrentOS() string {
	return runtime.GOOS
}

// SupportsUseKeychain reports whether the given OS understands the Apple-only
// UseKeychain ssh_config directive. It is true only on darwin. The sshconfig
// renderer (SSH-03) consults this to decide whether to emit the macOS keychain
// block inside Host *.
func SupportsUseKeychain(os string) bool {
	return os == "darwin"
}

// ProbeKeyTypes runs `ssh -Q key` and returns the supported key-type tokens
// (e.g. "ssh-ed25519", "ecdsa-sha2-nistp256", "ssh-rsa").
//
// The probe is the arg-slice form of exec.Command with no shell and no
// user-controlled arguments, keeping it free of OS-command-injection risk
// (gosec G204, threat T-02-06). It deliberately uses the `ssh` binary's
// `-Q key` query, NOT the keygen tool's `-Q key` (which is KRL-query mode and
// always errors) — see RESEARCH Pitfall 1, threat T-02-07.
func ProbeKeyTypes() ([]string, error) {
	out, err := exec.Command("ssh", "-Q", "key").Output() // #nosec G204 -- fixed args, no user input
	if err != nil {
		return nil, fmt.Errorf("probing ssh key types via `ssh -Q key`: %w", err)
	}
	return parseKeyTypes(string(out)), nil
}

// parseKeyTypes is the pure, testable core of ProbeKeyTypes: it splits the
// `ssh -Q key` output into trimmed, non-empty tokens.
func parseKeyTypes(out string) []string {
	lines := strings.Split(out, "\n")
	tokens := make([]string, 0, len(lines))
	for _, line := range lines {
		tok := strings.TrimSpace(line)
		if tok == "" {
			continue
		}
		tokens = append(tokens, tok)
	}
	return tokens
}

// SelectAlgorithm walks the fixed fallback chain (ed25519 -> rsa -> ecdsa) as
// membership tests against the supported token slice and returns the chosen
// algorithm name.
//
//   - ed25519 selected            -> warned == false
//   - any non-ed25519 selection   -> warned == true (the orchestrator surfaces
//     this downgrade to the user; threat T-02-08)
//   - none of the chain available -> error carrying per-OS install guidance
//     so the failure is actionable rather than opaque (D-14).
func SelectAlgorithm(supported []string) (algo string, warned bool, err error) {
	present := make(map[string]bool, len(supported))
	for _, tok := range supported {
		present[tok] = true
	}

	for i, candidate := range fallbackChain {
		if present[candidate.queryToken] {
			// warned is true for every selection past the first (ed25519).
			return candidate.name, i > 0, nil
		}
	}

	return "", false, fmt.Errorf(
		"no supported key algorithm (ed25519, rsa, or ecdsa) found in the local OpenSSH toolchain.\n%s",
		InstallHint(CurrentOS()),
	)
}

// InstallHint returns per-OS OpenSSH install/upgrade guidance. This is the
// mini-DOC-01 seam the Phase-4 doctor will generalize (D-14). Unknown operating
// systems fall back to the OpenSSH project link so the guidance is never empty.
func InstallHint(os string) string {
	const projectLink = "See https://www.openssh.com/ for source and platform install instructions."
	switch os {
	case "darwin":
		return "Install or upgrade OpenSSH with Homebrew: `brew install openssh`.\n" + projectLink
	case "linux":
		return "Install or upgrade OpenSSH with your package manager:\n" +
			"  Debian/Ubuntu: `sudo apt install openssh-client`\n" +
			"  Fedora/RHEL:   `sudo dnf install openssh-clients`\n" +
			"  Arch:          `sudo pacman -S openssh`\n" +
			projectLink
	default:
		return "Install or upgrade OpenSSH for your platform.\n" + projectLink
	}
}
