package gitconfig

import (
	"fmt"
	"os"
	"strings"

	"github.com/castocolina/gitid/internal/filewriter"
)

// gitconfigMode is the standard mode for ~/.gitconfig (and fragments). Unlike
// keys/configs under ~/.ssh, the gitconfig is not secret (Pitfall 6).
const gitconfigMode os.FileMode = 0o644

// MatchKind enumerates the includeIf selection strategies gitid renders.
type MatchKind int

const (
	// MatchGitdir selects an identity by repository directory: gitdir:~/git/<id>/.
	MatchGitdir MatchKind = iota
	// MatchHasconfig selects an identity by remote URL: hasconfig:remote.*.url:...
	MatchHasconfig
)

// Match is a single includeIf selection rule. Both kinds are combinable within
// one managed block for the same identity (GIT-02).
type Match struct {
	Kind  MatchKind
	Value string
}

// condition renders the includeIf condition string for a match, normalizing a
// gitdir value to the mandatory trailing slash (Pitfall 7 / D-13).
func (m Match) condition() string {
	switch m.Kind {
	case MatchGitdir:
		v := m.Value
		if !strings.HasSuffix(v, "/") {
			v += "/"
		}
		return "gitdir:" + v
	case MatchHasconfig:
		return "hasconfig:" + m.Value
	default:
		return m.Value
	}
}

// RenderIncludeIf builds the full managed-block text for an identity's includeIf
// headers, wrapped in `# BEGIN gitid managed: <identity>` / `# END gitid managed:
// <identity>` sentinels. Each match becomes an `[includeIf "<condition>"]` header
// followed by a `path = <fragment>` line. The gitdir condition always carries a
// trailing slash (GIT-02, Pitfall 7).
//
// A match Value (or the identity / fragment path) containing a newline could
// break out of the managed block and inject foreign git directives; such input
// is rejected with a panic, since callers pass gitid-derived, validated values
// and a newline here is a programming error, not user data.
func RenderIncludeIf(identity, fragmentPath string, matches []Match) string {
	body := renderBlockBody(identity, fragmentPath, matches)
	return filewriter.BeginPrefix + identity + "\n" + body + "\n" + filewriter.EndPrefix + identity
}

// renderBlockBody builds just the includeIf header/path lines (no sentinels),
// for use with filewriter.ReplaceBlock which supplies its own canonical markers.
func renderBlockBody(identity, fragmentPath string, matches []Match) string {
	for _, s := range []string{identity, fragmentPath} {
		if strings.ContainsAny(s, "\n\r") {
			panic("gitconfig: identity/fragment path must not contain newlines")
		}
	}
	var b strings.Builder
	for _, m := range matches {
		if strings.ContainsAny(m.Value, "\n\r") {
			panic("gitconfig: includeIf match value must not contain newlines")
		}
		fmt.Fprintf(&b, "[includeIf %q]\n", m.condition())
		fmt.Fprintf(&b, "\tpath = %s\n", fragmentPath)
	}
	return strings.TrimRight(b.String(), "\n")
}

// WriteIncludeIf composes the identity's includeIf managed block into
// gitconfigPath through the filewriter chokepoint (backup + atomic write +
// explicit 0644). It is idempotent: re-running with the same arguments leaves
// the file byte-identical, and all foreign content outside the managed block is
// preserved. It returns the backup path (empty when the target did not pre-exist).
func WriteIncludeIf(gitconfigPath, identity, fragmentPath string, matches []Match) (string, error) {
	body := renderBlockBody(identity, fragmentPath, matches)

	existing, err := os.ReadFile(gitconfigPath) //nolint:gosec // gitconfigPath is a trusted gitid-managed path
	if err != nil && !os.IsNotExist(err) {
		return "", fmt.Errorf("reading %s: %w", gitconfigPath, err)
	}

	composed := filewriter.ReplaceBlock(existing, identity, body)
	backupPath, err := filewriter.Write(gitconfigPath, composed, gitconfigMode)
	if err != nil {
		return "", fmt.Errorf("writing includeIf block to %s: %w", gitconfigPath, err)
	}
	return backupPath, nil
}
