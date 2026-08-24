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

// condition renders the includeIf condition string for a validated match,
// normalizing a gitdir value to the mandatory trailing slash (Pitfall 7 / D-13).
func (m Match) condition() string {
	if m.Kind == MatchGitdir {
		if strings.HasSuffix(m.Value, "/") {
			return "gitdir:" + m.Value
		}
		return "gitdir:" + m.Value + "/"
	}
	return "hasconfig:" + m.Value
}

// RenderIncludeIf builds the full managed-block text for an identity's includeIf
// headers, wrapped in `# BEGIN gitid managed: <identity>` / `# END gitid managed:
// <identity>` sentinels. Invalid form-derived input returns an empty preview.
func RenderIncludeIf(identity, fragmentPath string, matches []Match) string {
	block, err := RenderCheckedIncludeIf(identity, fragmentPath, matches)
	if err != nil {
		return ""
	}
	return block
}

// RenderCheckedIncludeIf builds the managed includeIf block after validating all
// values that shape Git config syntax.
func RenderCheckedIncludeIf(identity, fragmentPath string, matches []Match) (string, error) {
	if err := validateIncludeIf(identity, fragmentPath, matches); err != nil {
		return "", err
	}
	body := renderBlockBody(fragmentPath, matches)
	return filewriter.BeginPrefix + identity + "\n" + body + "\n" + filewriter.EndPrefix + identity, nil
}

// renderBlockBody builds just the includeIf header/path lines (no sentinels),
// for use with filewriter.ReplaceBlock which supplies its own canonical markers.
func renderBlockBody(fragmentPath string, matches []Match) string {
	var b strings.Builder
	for _, m := range matches {
		fmt.Fprintf(&b, "[includeIf %q]\n", m.condition())
		fmt.Fprintf(&b, "\tpath = %s\n", fragmentPath)
	}
	return strings.TrimRight(b.String(), "\n")
}

func validateIncludeIf(identity, fragmentPath string, matches []Match) error {
	if !safeInline(identity) || !safeInline(fragmentPath) || len(matches) == 0 {
		return fmt.Errorf("gitconfig: identity, fragment path, and at least one match are required")
	}
	for _, match := range matches {
		if !safeInline(match.Value) {
			return fmt.Errorf("gitconfig: includeIf match value contains unsafe characters")
		}
		switch match.Kind {
		case MatchGitdir:
			if match.Value == "" {
				return fmt.Errorf("gitconfig: gitdir match value is required")
			}
		case MatchHasconfig:
			if !validSSHHasconfig(match.Value) {
				return fmt.Errorf("gitconfig: hasconfig match must be SSH-only")
			}
		default:
			return fmt.Errorf("gitconfig: unknown includeIf match kind")
		}
	}
	return nil
}

func validSSHHasconfig(value string) bool {
	const prefix = "remote.*.url:git@"
	const suffix = ":*/**"
	if !strings.HasPrefix(value, prefix) || !strings.HasSuffix(value, suffix) {
		return false
	}
	host := strings.TrimSuffix(strings.TrimPrefix(value, prefix), suffix)
	if host == "" {
		return false
	}
	for _, r := range host {
		if (r < 'a' || r > 'z') && (r < 'A' || r > 'Z') && (r < '0' || r > '9') && r != '.' && r != '-' && r != '_' {
			return false
		}
	}
	return true
}

func safeInline(value string) bool {
	if value == "" {
		return false
	}
	for _, r := range value {
		if r < 0x20 || r == 0x7f {
			return false
		}
	}
	return true
}

// WriteIncludeIf composes the identity's includeIf managed block into
// gitconfigPath through the filewriter chokepoint (backup + atomic write +
// explicit 0644). It is idempotent: re-running with the same arguments leaves
// the file byte-identical, and all foreign content outside the managed block is
// preserved. It returns the backup path (empty when the target did not pre-exist).
func WriteIncludeIf(gitconfigPath, identity, fragmentPath string, matches []Match) (string, error) {
	if err := validateIncludeIf(identity, fragmentPath, matches); err != nil {
		return "", err
	}
	body := renderBlockBody(fragmentPath, matches)

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

const providerRewritePrefix = "provider-rewrite:"

// ProviderRewriteBlockName returns the provider-owned managed block name for a
// validated provider hostname. It is never identity-keyed, so identities sharing
// a provider share one rewrite block.
func ProviderRewriteBlockName(provider string) (string, error) {
	host, err := validProviderHostname(provider)
	if err != nil {
		return "", err
	}
	return providerRewritePrefix + host, nil
}

// RenderProviderRewrite renders the recipe-shaped HTTPS-to-SSH URL rewrite for
// one validated provider hostname.
func RenderProviderRewrite(provider string) (string, error) {
	host, err := validProviderHostname(provider)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("[url %q]\n\tinsteadOf = https://%s/", "git@"+host+":", host), nil
}

// WriteProviderRewrite writes one provider-owned HTTPS-to-SSH rewrite through
// the managed-block chokepoint. When enabled is false, it intentionally does
// nothing: D-06 opt-out must never remove a rewrite another identity manages.
func WriteProviderRewrite(gitconfigPath, provider string, enabled bool) (string, error) {
	name, err := ProviderRewriteBlockName(provider)
	if err != nil {
		return "", err
	}
	if !enabled {
		return "", nil
	}
	body, err := RenderProviderRewrite(provider)
	if err != nil {
		return "", err
	}

	existing, err := os.ReadFile(gitconfigPath) //nolint:gosec // gitconfigPath is a trusted gitid-managed path
	if err != nil && !os.IsNotExist(err) {
		return "", fmt.Errorf("reading %s: %w", gitconfigPath, err)
	}
	backupPath, err := filewriter.Write(gitconfigPath, filewriter.ReplaceBlock(existing, name, body), gitconfigMode)
	if err != nil {
		return "", fmt.Errorf("writing provider rewrite block to %s: %w", gitconfigPath, err)
	}
	return backupPath, nil
}

func validProviderHostname(provider string) (string, error) {
	host := strings.ToLower(provider)
	if len(host) == 0 || len(host) > 253 || strings.HasSuffix(host, ".") {
		return "", fmt.Errorf("gitconfig: provider hostname is invalid")
	}
	labels := strings.Split(host, ".")
	if len(labels) < 2 {
		return "", fmt.Errorf("gitconfig: provider hostname is invalid")
	}
	for _, label := range labels {
		if len(label) == 0 || len(label) > 63 || label[0] == '-' || label[len(label)-1] == '-' {
			return "", fmt.Errorf("gitconfig: provider hostname is invalid")
		}
		for _, r := range label {
			if (r < 'a' || r > 'z') && (r < '0' || r > '9') && r != '-' {
				return "", fmt.Errorf("gitconfig: provider hostname is invalid")
			}
		}
	}
	return host, nil
}
