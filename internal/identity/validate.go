package identity

import (
	"fmt"
	"regexp"
	"strings"
)

// nameRe is the allowed charset for a gitid identity name: letters, digits,
// dot, underscore, and hyphen. It rejects whitespace and shell/newline
// metacharacters so the name can never inject into an arg-slice exec or break
// a managed block (T-05-01).
var nameRe = regexp.MustCompile(`^[A-Za-z0-9._-]+$`)

// ValidateName validates an identity name against the allowed charset.
// It trims surrounding whitespace first, then rejects empty names and names
// that do not match ^[A-Za-z0-9._-]+$.
func ValidateName(name string) error {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return fmt.Errorf("identity name is required")
	}
	if !nameRe.MatchString(trimmed) {
		return fmt.Errorf("invalid identity name %q: only letters, digits, dot, underscore, and hyphen are allowed", trimmed)
	}
	// Reject if original name had leading/trailing whitespace (already rejected
	// above via empty check, but check for non-empty names with surrounding space).
	if trimmed != name {
		return fmt.Errorf("invalid identity name %q: must not have leading or trailing whitespace", name)
	}
	return nil
}

// ValidateEmail validates a git user.email value at the form/edit boundary so a
// malformed address is rejected EARLY — before key generation and the SSH test —
// rather than failing deep inside the fragment write. It mirrors the write-time
// rule in gitconfig.validateEmail (no newlines, no embedded spaces/tabs, must
// contain "@"); unlike a name, an email's local part forbids whitespace. Leading
// or trailing whitespace is rejected explicitly so a stray paste is caught.
func ValidateEmail(email string) error {
	trimmed := strings.TrimSpace(email)
	if trimmed == "" {
		return fmt.Errorf("git email is required")
	}
	if trimmed != email {
		return fmt.Errorf("invalid email %q: must not have leading or trailing whitespace", email)
	}
	if strings.ContainsAny(email, "\n\r") {
		return fmt.Errorf("invalid email %q: must not contain newlines", email)
	}
	if strings.ContainsAny(email, " \t") || !strings.Contains(email, "@") {
		return fmt.Errorf("invalid email %q: must be a single address containing '@' with no spaces", email)
	}
	return nil
}

// ValidateProvider validates a provider value against the same safe charset as
// an identity name. The provider is written verbatim into ~/.ssh/config as a
// `# gitid: provider=<p>` marker (D-11) and is used to build default hostnames
// and URL patterns, so it must reject whitespace and newline/metacharacters that
// would break the marker round-trip or inject into a Host block (CR-01). An empty
// provider is allowed — it is optional and simply omits the marker.
func ValidateProvider(provider string) error {
	if provider == "" {
		return nil
	}
	if strings.TrimSpace(provider) != provider {
		return fmt.Errorf("invalid provider %q: must not have leading or trailing whitespace", provider)
	}
	if !nameRe.MatchString(provider) {
		return fmt.Errorf("invalid provider %q: only letters, digits, dot, underscore, and hyphen are allowed", provider)
	}
	return nil
}
