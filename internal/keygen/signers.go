package keygen

import (
	"fmt"
	"os"
	"strings"

	"github.com/castocolina/gitid/internal/filewriter"
)

// allowedSignersMode is the mode for ~/.ssh/allowed_signers: readable, not
// secret (RESEARCH Pitfall 6).
const allowedSignersMode = 0o644

// AllowedSignersLine builds an allowed_signers line for git SSH signing
// (SIGN-01): `<email> namespaces="git" <pubLine-without-trailing-newline>\n`.
//
// MarshalAuthorizedKey appends a trailing newline; it is stripped here and a
// single newline re-added so the composed line never carries a double newline.
// The email is used byte-identically to the supplied value (Pitfall 8).
func AllowedSignersLine(email, pubLine string) string {
	keyText := strings.TrimRight(pubLine, "\n")
	return fmt.Sprintf("%s namespaces=\"git\" %s\n", email, keyText)
}

// WriteAllowedSigners persists line into the allowed_signers file at path as an
// idempotent per-identity managed block keyed by identity (SAFE-02). Existing
// content is read (empty if absent), the per-identity block is spliced via
// filewriter.ReplaceBlock, and the result is written through filewriter at mode
// 0644. Re-running with the same identity+line yields an empty diff; a different
// identity appends a distinct block while preserving foreign content.
//
// path is a trusted, gitid-managed path supplied in-process. It returns the
// backup path produced by filewriter when the file pre-existed.
func WriteAllowedSigners(path, identity, line string) (string, error) {
	existing, err := os.ReadFile(path) //nolint:gosec // path is a trusted gitid-managed path
	if err != nil && !os.IsNotExist(err) {
		return "", fmt.Errorf("keygen: reading %s: %w", path, err)
	}

	composed := filewriter.ReplaceBlock(existing, identity, strings.TrimRight(line, "\n"))

	backup, err := filewriter.Write(path, composed, allowedSignersMode)
	if err != nil {
		return "", fmt.Errorf("keygen: writing allowed_signers: %w", err)
	}
	return backup, nil
}
