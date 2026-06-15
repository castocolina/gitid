// Package upload provides the provider-specific instructions for uploading an
// SSH public key to a Git hosting provider. It is imported by both cmd/gitid
// and tui so that neither package duplicates the instruction strings.
package upload

import (
	"fmt"
	"strings"
)

// Instructions returns the provider-specific steps for uploading a public key.
// Extracted from cmd/gitid/upload.go so both cmd/gitid/copy.go and tui/copy.go
// can import it without an import cycle.
//
// The same .pub serves both authentication and commit signing, but providers
// register it differently:
//
//   - GitHub: TWO SEPARATE registrations at https://github.com/settings/ssh/new
//     — add the identical .pub once with key type "Authentication key" and again
//     with key type "Signing key". GitHub does not let one key serve both roles.
//   - GitLab: ONE key at https://gitlab.com/-/user_settings/ssh_keys with
//     "Usage type" set to "Authentication & Signing", so a single registration
//     covers both.
//
// Unknown providers get a generic instruction so the output is never empty.
func Instructions(provider string) string {
	switch strings.ToLower(provider) {
	case "github":
		var b strings.Builder
		b.WriteString("Upload your public key to GitHub (TWO separate registrations of the SAME key):\n")
		b.WriteString("  1. Open https://github.com/settings/ssh/new\n")
		b.WriteString("  2. Authentication key: paste the .pub, set \"Key type\" = Authentication key, Add SSH key.\n")
		b.WriteString("  3. Open https://github.com/settings/ssh/new again.\n")
		b.WriteString("  4. Signing key: paste the SAME .pub, set \"Key type\" = Signing key, Add SSH key.\n")
		b.WriteString("GitHub requires the key registered twice — once for authentication, once for signing.\n")
		return b.String()
	case "gitlab":
		var b strings.Builder
		b.WriteString("Upload your public key to GitLab (ONE key covers both roles):\n")
		b.WriteString("  1. Open https://gitlab.com/-/user_settings/ssh_keys\n")
		b.WriteString("  2. Paste the .pub, set \"Usage type\" = Authentication & Signing, Add key.\n")
		return b.String()
	default:
		return fmt.Sprintf(
			"Upload your public key to %s as both an authentication key and a signing key,\n"+
				"following that provider's SSH key settings page.\n", provider)
	}
}
