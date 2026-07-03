package platform

import (
	"context"
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
// The probe is the arg-slice form of exec.CommandContext with no shell and no
// user-controlled arguments, keeping it free of OS-command-injection risk
// (gosec G204, threat T-02-06). It deliberately uses the `ssh` binary's
// `-Q key` query, NOT the keygen tool's `-Q key` (which is KRL-query mode and
// always errors) — see RESEARCH Pitfall 1, threat T-02-07. The probe runs
// under a bounded timeout (T-01-03: a hung `ssh` binary must never block
// gitid).
func ProbeKeyTypes() ([]string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), probeTimeout)
	defer cancel()
	out, err := exec.CommandContext(ctx, "ssh", "-Q", "key").Output() // #nosec G204 -- fixed args, no user input
	if err != nil {
		return nil, fmt.Errorf("platform: probing ssh key types via `ssh -Q key`: %w", err)
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
		InstallHint("openssh", CurrentOS()),
	)
}

// InstallHint returns per-OS install/upgrade guidance for the named tool.
// The tool parameter identifies the tool family:
//
//   - "openssh" or "ssh" or "ssh-keygen" or "ssh-add": OpenSSH suite
//   - "git": git version control system
//   - "clipboard": platform clipboard helper (pbcopy/xclip/wl-copy)
//
// When os is "darwin", only the Homebrew line is returned (single-platform hint).
// When os is "linux", all three Linux package-manager lines are returned.
// When os is unknown, all four package-manager lines (brew + apt + dnf + pacman)
// are returned so the output is actionable regardless of platform (DOC-01).
//
// Unknown tools fall back to OpenSSH guidance so that existing callers of the
// original single-parameter style remain actionable.
func InstallHint(tool, os string) string {
	switch normalizeTool(tool) {
	case "git":
		return gitInstallHint(os)
	case "clipboard":
		return clipboardInstallHint(os)
	case "libfido2":
		return libfido2InstallHint(os)
	default:
		// openssh, ssh, ssh-keygen, ssh-add, or unknown tool → OpenSSH guidance.
		return opensshInstallHint(os)
	}
}

// normalizeTool maps tool name variants to canonical keys used in InstallHint.
func normalizeTool(tool string) string {
	switch tool {
	case "git":
		return "git"
	case "clipboard", "pbcopy", "xclip", "wl-copy", "xsel":
		return "clipboard"
	case "libfido2", "ssh-sk-helper":
		return "libfido2"
	default:
		return "openssh"
	}
}

// libfido2InstallHint returns per-OS libfido2/FIDO2 hardware-key install
// guidance (KEY-03).
// When os is "darwin", only the Homebrew line is returned.
// When os is "linux", all three Linux package-manager lines are returned.
// Unknown OS: all four lines are returned.
func libfido2InstallHint(os string) string {
	const projectLink = "See https://developers.yubico.com/libfido2/ for source and platform install instructions."
	switch os {
	case "darwin":
		return "brew install libfido2  (macOS)\n" + projectLink
	case "linux":
		return "Install libfido2 with your package manager:\n" +
			"  apt install libfido2-1 libfido2-dev  (Debian/Ubuntu)\n" +
			"  dnf install libfido2                 (Fedora)\n" +
			"  pacman -S libfido2                   (Arch)\n" +
			projectLink
	default:
		return "Install libfido2 with your package manager:\n" +
			"  brew install libfido2                (macOS)\n" +
			"  apt install libfido2-1 libfido2-dev  (Debian/Ubuntu)\n" +
			"  dnf install libfido2                 (Fedora)\n" +
			"  pacman -S libfido2                   (Arch)\n" +
			projectLink
	}
}

// opensshInstallHint returns per-OS OpenSSH install guidance.
// When os is "darwin", only the Homebrew line is returned.
// When os is "linux", all three Linux package-manager lines are returned.
// Unknown OS: all four lines are returned.
func opensshInstallHint(os string) string {
	const projectLink = "See https://www.openssh.com/ for source and platform install instructions."
	switch os {
	case "darwin":
		return "brew install openssh  (macOS)\n" + projectLink
	case "linux":
		return "Install or upgrade OpenSSH with your package manager:\n" +
			"  apt install openssh-client   (Debian/Ubuntu)\n" +
			"  dnf install openssh-clients  (Fedora)\n" +
			"  pacman -S openssh            (Arch)\n" +
			projectLink
	default:
		return "Install or upgrade OpenSSH with your package manager:\n" +
			"  brew install openssh         (macOS)\n" +
			"  apt install openssh-client   (Debian/Ubuntu)\n" +
			"  dnf install openssh-clients  (Fedora)\n" +
			"  pacman -S openssh            (Arch)\n" +
			projectLink
	}
}

// gitInstallHint returns per-OS git install guidance.
// When os is "darwin", only the Homebrew line is returned.
// When os is "linux", all three Linux package-manager lines are returned.
// Unknown OS: all four lines are returned.
func gitInstallHint(os string) string {
	const projectLink = "See https://git-scm.com/ for source and platform install instructions."
	switch os {
	case "darwin":
		return "brew install git  (macOS)\n" + projectLink
	case "linux":
		return "Install or upgrade git with your package manager:\n" +
			"  apt install git   (Debian/Ubuntu)\n" +
			"  dnf install git   (Fedora)\n" +
			"  pacman -S git     (Arch)\n" +
			projectLink
	default:
		return "Install or upgrade git with your package manager:\n" +
			"  brew install git  (macOS)\n" +
			"  apt install git   (Debian/Ubuntu)\n" +
			"  dnf install git   (Fedora)\n" +
			"  pacman -S git     (Arch)\n" +
			projectLink
	}
}

// clipboardInstallHint returns per-OS clipboard helper install guidance.
// On macOS, pbcopy is bundled with the OS — brew install pbcopy is the fallback
// for shells that lack it. On Linux, xclip is the recommended tool.
// When os is "darwin", only the macOS line is returned.
// When os is "linux", only the Linux lines are returned.
// Unknown OS: all four lines are returned.
func clipboardInstallHint(os string) string {
	switch os {
	case "darwin":
		return "brew install pbcopy  (macOS — included with macOS, try reinstalling if missing)"
	case "linux":
		return "Install a clipboard helper with your package manager:\n" +
			"  apt install xclip   (Debian/Ubuntu)\n" +
			"  dnf install xclip   (Fedora)\n" +
			"  pacman -S xclip     (Arch)"
	default:
		return "Install a clipboard helper with your package manager:\n" +
			"  brew install pbcopy  (macOS)\n" +
			"  apt install xclip    (Debian/Ubuntu)\n" +
			"  dnf install xclip    (Fedora)\n" +
			"  pacman -S xclip      (Arch)"
	}
}
