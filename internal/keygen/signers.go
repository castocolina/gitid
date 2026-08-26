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
// (SIGN-01): `<email> namespaces="git" <keytype> <base64-key>\n`.
//
// Only the first two fields of the public line (keytype + base64 key) are kept:
// the pub line may now carry a trailing comment ("ssh-ed25519 AAAA… work@gitid"),
// which must NOT bleed into the signer line — the principal there is the email.
// The email is used byte-identically to the supplied value (Pitfall 8).
//
// CR-18/WR-08: ssh-keygen(1)'s allowed_signers format treats the PRINCIPALS
// field as a comma-separated list, so an email carrying a bare comma
// smuggles in an attacker-chosen second principal (e.g. "victim@corp.test,*"
// grants a wildcard match — verified against real `ssh-keygen -Y verify`).
// This is the write-time hard gate: it fails closed rather than emit a
// multi-principal (or multi-line, or reshaped-field) line, independent of
// whether an upstream form/config validator already rejected the offending
// character (a pre-existing fragment read back for reuse/adopt never
// re-runs that validation). WR-08: a comma is not the only character that
// breaks this contract — a newline in email injects an entire ADDITIONAL
// allowed_signers line (an attacker-chosen principal + key on its own
// line), and a space or embedded "namespaces=" silently changes the field
// layout ssh-keygen(1) parses. The gate now rejects every character that
// is unsafe in this single-line, single-field format, and requires a
// single bare "user@host"-shaped address — never partial validation that
// relies on some earlier, independent layer having already run.
func AllowedSignersLine(email, pubLine string) (string, error) {
	if email == "" || strings.ContainsAny(email, ",\n\r \t") || !strings.Contains(email, "@") {
		return "", fmt.Errorf("keygen: allowed_signers principal is not a single bare address (CR-18): %q", email)
	}
	keyText := strings.TrimRight(pubLine, "\n")
	if fields := strings.Fields(keyText); len(fields) >= 2 {
		keyText = fields[0] + " " + fields[1]
	}
	return fmt.Sprintf("%s namespaces=\"git\" %s\n", email, keyText), nil
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

// WriteAllowedSignersReplacing replaces identity's managed signer block with
// exactly one signer built from the exact user.email bytes. It never appends a
// second principal for the same identity.
func WriteAllowedSignersReplacing(path, identity, email, pubLine string) (string, error) {
	line, err := AllowedSignersLine(email, pubLine)
	if err != nil {
		return "", err
	}
	return WriteAllowedSigners(path, identity, line)
}

// AppendAllowedSigners appends pubLine's signer line for email to identity's
// managed allowed_signers block WITHOUT dropping the block's existing
// line(s) (D-07). This is the KEY-CEREMONY writer used by BOTH rotate and
// repair (rotate: D-07; plan 05-03 records the planner resolution extending
// the same rule to repair). create/update/clone keep the single-line
// WriteAllowedSigners/WriteAllowedSignersReplacing semantics — only rotate
// and repair need the OLD line to survive.
//
// Reason: an allowed_signers entry verifies signatures from its PUBLIC key
// blob alone, so keeping the old line lets `git log --show-signature` keep
// verifying pre-rotation commits even after the old private key is archived
// or gone. Pruning accumulated lines is deliberately deferred to Phase 8.
//
// The new line is built through AllowedSignersLine, so the CR-18
// comma-injection guard applies to this write path exactly as it applies to
// the existing one — never construct the line string inline. A comma in
// email is rejected before any read or write, leaving the file untouched.
//
// Appending a line already present in the block (compared exactly, after
// trimming) is a no-op: it returns an empty backup path and a nil error
// rather than re-writing the file, so a byte-identical re-run never
// produces a spurious backup.
//
// N rotations leave N+1 lines in one identity's block (review R-22): the
// removal side needs no change because RemoveAllowedSignersBlock is
// block-keyed (by identity name), not line-keyed — deleting the whole
// identity removes all N+1 lines together regardless of count, and a
// Git-only delete keeps the whole block regardless of count (D-10).
func AppendAllowedSigners(path, identity, email, pubLine string) (backupPath string, err error) {
	line, err := AllowedSignersLine(email, pubLine)
	if err != nil {
		return "", err
	}
	newLine := strings.TrimRight(line, "\n")

	existing, err := os.ReadFile(path) //nolint:gosec // path is a trusted gitid-managed path
	if err != nil && !os.IsNotExist(err) {
		return "", fmt.Errorf("keygen: reading %s: %w", path, err)
	}

	var body string
	for _, block := range filewriter.ListBlocks(existing) {
		if block.Name == identity {
			body = block.Body
			break
		}
	}

	for _, existingLine := range strings.Split(body, "\n") {
		if existingLine == newLine {
			return "", nil // already present — idempotent no-op
		}
	}

	combined := newLine
	if body != "" {
		combined = body + "\n" + newLine
	}

	composed := filewriter.ReplaceBlock(existing, identity, combined)
	backup, err := filewriter.Write(path, composed, allowedSignersMode)
	if err != nil {
		return "", fmt.Errorf("keygen: appending allowed_signers: %w", err)
	}
	return backup, nil
}
