package sshconfig

import (
	"fmt"
	"strings"
	"unicode"
)

// hostIndent is the two-space indentation OpenSSH config conventionally uses for
// directives nested under a Host stanza.
const hostIndent = "  "

// RenderHostBlock renders a managed SSH Host stanza for an identity.
//
// It emits, in order (SSH-01): the Host line for alias, then Hostname, Port,
// `User git`, IdentityFile, and `IdentitiesOnly yes`. When provider is
// non-empty, appends a `# gitid: provider=<p>` marker comment as the last line
// of the stanza body (D-11). provider must not contain a newline or carriage
// return — RenderHostBlock panics on such input (T-05.5-04).
//
// The alias is the real provider host for a default identity or an
// `<identity>.<provider>` alias for an additional identity (SSH-02); both forms
// render identically here.
//
// `IdentitiesOnly yes` together with the explicit IdentityFile prevents the
// agent offering the wrong key to the provider (T-02-13).
//
// The returned text is the block BODY only (no sentinel markers); the writer
// wraps it in a gitid managed block keyed by the identity name.
func RenderHostBlock(alias, hostname string, port int, identityFile, provider string) string {
	if strings.ContainsAny(provider, "\n\r") {
		panic("sshconfig: RenderHostBlock: provider must not contain newline or carriage return (T-05.5-04)")
	}
	var b strings.Builder
	fmt.Fprintf(&b, "Host %s\n", alias)
	fmt.Fprintf(&b, "%sHostname %s\n", hostIndent, hostname)
	fmt.Fprintf(&b, "%sPort %d\n", hostIndent, port)
	fmt.Fprintf(&b, "%sUser git\n", hostIndent)
	fmt.Fprintf(&b, "%sIdentityFile %s\n", hostIndent, identityFile)
	fmt.Fprintf(&b, "%sIdentitiesOnly yes\n", hostIndent)
	if provider != "" {
		fmt.Fprintf(&b, "# gitid: provider=%s\n", provider)
	}
	return b.String()
}

// validateIdentityFileStrict applies the CR-08 final-boundary check: rejects any
// character for which unicode.IsSpace returns true, any Unicode control character
// (unicode.IsControl), plus the syntax-special characters already rejected by
// ValidateHostBlock. This is stricter than ValidateHostBlock's ASCII-only
// whitespace check and catches e.g. U+00A0 NO-BREAK SPACE and U+0085 NEL.
func validateIdentityFileStrict(identityFile string) error {
	if strings.TrimSpace(identityFile) == "" {
		return fmt.Errorf("IdentityFile cannot be empty")
	}
	for _, r := range identityFile {
		switch {
		case r == 0:
			return fmt.Errorf("IdentityFile contains NUL character")
		case r == '\n' || r == '\r':
			return fmt.Errorf("IdentityFile contains line break")
		case unicode.IsSpace(r):
			return fmt.Errorf("IdentityFile contains unicode whitespace U+%04X — unquoted OpenSSH tokens may not contain whitespace", r)
		case unicode.IsControl(r):
			return fmt.Errorf("IdentityFile contains unicode control character U+%04X", r)
		case r == '"' || r == '\'':
			return fmt.Errorf("IdentityFile contains a quote character — unquoted tokens may not contain quotes")
		case r == '\\':
			return fmt.Errorf("IdentityFile contains a backslash — ambiguous escaping in unquoted tokens is not supported")
		case r == '#':
			return fmt.Errorf("IdentityFile contains '#' — interpreted as a comment separator in SSH config")
		case r == '=':
			return fmt.Errorf("IdentityFile contains '=' — interpreted as a key-value separator in SSH config")
		}
	}
	return nil
}

// RenderCheckedHostBlock is the authoritative render boundary for an SSH Host
// stanza (CR-08). It validates every field — including unicode.IsSpace and
// Unicode controls for IdentityFile — returns an error for any invalid input,
// and only then delegates to RenderHostBlock. All production callers that
// write or preview a Host block MUST use this function; direct calls to
// RenderHostBlock are reserved for internal tests of the render shape itself.
//
// Callers: HostBlockPreview, StageTestConfig, commitCreateTransaction.
func RenderCheckedHostBlock(alias, hostname string, port int, identityFile, provider string) (string, error) {
	if err := validateIdentityFileStrict(identityFile); err != nil {
		return "", &ValidationError{Field: "identityFile", Message: err.Error()}
	}
	// Delegate alias/hostname/port validation to ValidateHostBlock; pass a
	// placeholder IdentityFile since we already validated it above and
	// ValidateHostBlock would duplicate the check.
	portStr := fmt.Sprintf("%d", port)
	if err := ValidateHostBlock(alias, hostname, portStr, "~/.ssh/placeholder"); err != nil {
		return "", err
	}
	return RenderHostBlock(alias, hostname, port, identityFile, provider), nil
}
