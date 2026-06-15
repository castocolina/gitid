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
