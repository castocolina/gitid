package gitconfig

import (
	"fmt"
	"os/exec"
	"strings"
)

// WriteFragment writes the per-identity gitconfig fragment at fragmentPath using
// `git config --file` (git is the authoritative parser of its own format, so the
// writes are idempotent and comment-safe). It sets exactly the identity-only keys:
//
//	user.name       = <name>
//	user.email      = <email>
//	gpg.format      = ssh
//	user.signingkey = <signingKeyPath>   (a .pub PATH, never an inline key — SIGN-02)
//	commit.gpgsign  = true
//
// The fragment must NOT contain a [remote] section: a remote URL in a fragment
// included via `hasconfig:` is a hard git circular error (Pitfall 9), so any
// value that would introduce one is rejected before any write occurs. All values
// are arg-slice arguments to exec.Command (no shell), keeping it gosec G204-clean
// and free of OS-command-injection risk (threat T-02-18).
func WriteFragment(fragmentPath, name, email, signingKeyPath string) error {
	if err := validateValue("user.name", name); err != nil {
		return err
	}
	if err := validateEmail(email); err != nil {
		return err
	}
	if err := validateValue("user.signingkey", signingKeyPath); err != nil {
		return err
	}

	settings := [][2]string{
		{"user.name", name},
		{"user.email", email},
		{"gpg.format", "ssh"},
		{"user.signingkey", signingKeyPath},
		{"commit.gpgsign", "true"},
	}
	for _, kv := range settings {
		if err := gitConfigSet(fragmentPath, kv[0], kv[1]); err != nil {
			return err
		}
	}
	return nil
}

// SetAllowedSignersFile sets the global gpg.ssh.allowedSignersFile in
// gitconfigPath so SSH-signed commits are verified against the gitid-managed
// allowed_signers file (SIGN-01). The value is a filesystem path written via the
// arg-slice `git config --file` form (gosec G204-clean).
func SetAllowedSignersFile(gitconfigPath, allowedSignersPath string) error {
	if err := validateValue("gpg.ssh.allowedSignersFile", allowedSignersPath); err != nil {
		return err
	}
	return gitConfigSet(gitconfigPath, "gpg.ssh.allowedSignersFile", allowedSignersPath)
}

// gitConfigSet runs `git config --file <path> <key> <value>` with arguments
// passed as a slice (never through a shell), so user-derived values cannot be
// interpreted as shell or git metacharacters.
func gitConfigSet(path, key, value string) error {
	cmd := exec.Command("git", "config", "--file", path, key, value) //nolint:gosec // arg-slice form, no shell; values validated above (G204)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git config --file %s %s: %w: %s", path, key, err, strings.TrimSpace(string(out)))
	}
	return nil
}

// validateValue rejects values that could break the fragment: embedded newlines
// (which could inject additional git directives, including a forbidden [remote]
// section) and any literal `[remote` token (Pitfall 9, threat T-02-20).
func validateValue(key, value string) error {
	if strings.ContainsAny(value, "\n\r") {
		return fmt.Errorf("gitconfig: %s must not contain newlines: %q", key, value)
	}
	if strings.Contains(strings.ToLower(value), "[remote") {
		return fmt.Errorf("gitconfig: %s must not introduce a [remote] section (hasconfig circular error)", key)
	}
	return nil
}

// validateEmail applies validateValue plus a minimal shape check so a clearly
// malformed address never reaches the fragment.
func validateEmail(email string) error {
	if err := validateValue("user.email", email); err != nil {
		return err
	}
	if !strings.Contains(email, "@") || strings.ContainsAny(email, " \t") {
		return fmt.Errorf("gitconfig: user.email is malformed: %q", email)
	}
	return nil
}
