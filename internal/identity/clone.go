package identity

import (
	"errors"
	"fmt"
	"path"
	"strings"

	"github.com/castocolina/gitid/internal/gitconfig"
	"github.com/castocolina/gitid/internal/sshconfig"
)

// CloneSuffix is the frozen D-17 suffix appended to a source identity name to
// form the first suggested clone name ("<source>-clone").
const CloneSuffix = "-clone"

// Copied-field identifiers reported by DeriveCloneInput. D-14 restricts the
// review flag to exactly these two author fields; every other field is
// re-derived and must not appear here.
const (
	CopiedFieldGitName  = "user.name"
	CopiedFieldGitEmail = "user.email"
)

// FieldError is a typed, field-keyed validation error so a prompt can render
// the failure inline on the offending control (mirrors sshconfig.ValidationError).
type FieldError struct {
	Field   string
	Message string
}

func (e *FieldError) Error() string { return e.Message }

// CloneTargets carries the resolved managed paths DeriveCloneInput writes into
// CreateInput, so the function stays free of platform lookups.
type CloneTargets struct {
	GitconfigPath      string
	SSHConfigPath      string
	AllowedSignersPath string
	FragmentDir        string
}

// CloneNotices reports which fields were copied verbatim so the wizard can
// attach the D-14 review flag to exactly those rows.
type CloneNotices struct {
	CopiedFields []string
}

// SuggestCloneName returns the D-17 suggested clone name: source + CloneSuffix,
// silently auto-bumped to the next free numbered variant when the candidate is
// present in taken (case-insensitive — SSH host patterns are matched
// case-insensitively). Callable with no I/O; the caller supplies taken.
func SuggestCloneName(source string, taken []string) string {
	base := source + CloneSuffix
	if !nameTaken(base, taken) {
		return base
	}
	for n := 2; ; n++ {
		candidate := fmt.Sprintf("%s-%d", base, n)
		if !nameTaken(candidate, taken) {
			return candidate
		}
	}
}

func nameTaken(candidate string, taken []string) bool {
	for _, name := range taken {
		if strings.EqualFold(name, candidate) {
			return true
		}
	}
	return false
}

// GitdirMatch rebuilds a gitdir includeIf match for cloneName via DefaultMatch,
// so there is one gitdir shape in the project (review R-12). Never substitute
// textually into a source Match.Value.
func GitdirMatch(cloneName string) gitconfig.Match {
	return DefaultMatch(cloneName)
}

// HasconfigMatch rebuilds a hasconfig includeIf match from the derived clone
// alias in the recipe-canonical shape
// hasconfig:remote.*.url:git@<alias>:*/** (review R-12).
func HasconfigMatch(alias string) gitconfig.Match {
	return gitconfig.Match{
		Kind:  gitconfig.MatchHasconfig,
		Value: "remote.*.url:git@" + alias + ":*/**",
	}
}

// DeriveCloneInput builds the CreateInput a clone feeds into the existing create
// wizard (D-14 / D-15). It does NOT write anything — the create path owns the
// test gate, collision block, ceremony, and rollback.
//
// Copy versus re-derive (explicit two-column decision — do not blur):
//
//	COPIED from source (and reported in CloneNotices.CopiedFields):
//	  GitName, GitEmail
//	COPIED from source (not flagged — not author fields):
//	  Provider, Hostname, Port
//	RE-DERIVED from the new name / key choice:
//	  Name, Alias (via DefaultAlias), Matches (by kind — see below),
//	  FragmentPath, ReuseKeyPath (source key when reuseSourceKey, else empty
//	  so the create path generates at the new canonical path)
//	NEVER copied:
//	  the allowed_signers line — the write pipeline rebuilds it from the new
//	  email and the new public line
//
// Matches are rebuilt by KIND only (review R-12). A gitdir source yields
// GitdirMatch(cloneName); a hasconfig source yields HasconfigMatch(derivedAlias).
// The source Match.Value is a fact about the SOURCE identity and carries no
// information about the clone — it is discarded entirely. Kind and slice
// position are preserved.
//
// Recipe divergence (review R-23): identity.DefaultMatch currently defaults to
// gitdir:~/git/<identity>/, while recipes/README.md names
// hasconfig:remote.*.url:git@<alias>:*/** as the PRIMARY match and gitdir as the
// alternative. That is an accepted, pre-existing project divergence. Clone
// PRESERVES the source's kind (so a hasconfig source stays recipe-canonical);
// a source with no match at all falls back to the project default gitdir.
func DeriveCloneInput(source Account, cloneName string, reuseSourceKey bool, targets CloneTargets) (CreateInput, CloneNotices, error) {
	cloneName = strings.TrimSpace(cloneName)
	notices := CloneNotices{
		CopiedFields: []string{CopiedFieldGitName, CopiedFieldGitEmail},
	}
	if cloneName == "" {
		return CreateInput{}, notices, &FieldError{Field: "name", Message: "clone name is required"}
	}
	if strings.EqualFold(cloneName, source.Name) {
		return CreateInput{}, notices, &FieldError{Field: "name", Message: "clone name must differ from the source name"}
	}
	if err := ValidateName(cloneName); err != nil {
		return CreateInput{}, notices, &FieldError{Field: "name", Message: err.Error()}
	}

	// Reconstruct stores a short provider token ("github") when no
	// "# gitid: provider=" marker is present. DefaultAlias concatenates
	// that token, which would write Host acme-clone.github. The TUI
	// wizard rebuilds the FQDN from the hostname; normalize here so the
	// CLI clone path produces the same alias.
	provider := source.Provider
	if fqdn := RewriteProviderKey(provider, source.Alias); fqdn != "" {
		provider = fqdn
	}
	alias := DefaultAlias(cloneName, provider)
	portStr := fmt.Sprintf("%d", source.Port)
	if source.Port == 0 {
		portStr = fmt.Sprintf("%d", DefaultPort())
	}
	// Same host-block validator the create path uses (T-05-22).
	keyForValidation := source.KeyPath
	if !reuseSourceKey || keyForValidation == "" {
		keyForValidation = "~/.ssh/id_ed25519_" + cloneName
	}
	if err := sshconfig.ValidateHostBlock(alias, source.Hostname, portStr, keyForValidation); err != nil {
		field := "alias"
		var ve *sshconfig.ValidationError
		if errors.As(err, &ve) {
			field = ve.Field
		}
		return CreateInput{}, notices, &FieldError{Field: field, Message: err.Error()}
	}

	matches := deriveCloneMatches(source.Matches, cloneName, alias)
	port := source.Port
	if port == 0 {
		port = DefaultPort()
	}

	in := CreateInput{
		Name:               cloneName,
		GitName:            source.GitName,
		GitEmail:           source.GitEmail,
		Provider:           provider,
		Alias:              alias,
		Hostname:           source.Hostname,
		Port:               port,
		Matches:            matches,
		FragmentPath:       path.Join(targets.FragmentDir, cloneName),
		GitconfigPath:      targets.GitconfigPath,
		SSHConfigPath:      targets.SSHConfigPath,
		AllowedSignersPath: targets.AllowedSignersPath,
	}
	if reuseSourceKey {
		in.ReuseKeyPath = source.KeyPath
	}
	return in, notices, nil
}

func deriveCloneMatches(source []gitconfig.Match, cloneName, alias string) []gitconfig.Match {
	if len(source) == 0 {
		return []gitconfig.Match{GitdirMatch(cloneName)}
	}
	out := make([]gitconfig.Match, 0, len(source))
	for _, m := range source {
		switch m.Kind {
		case gitconfig.MatchHasconfig:
			out = append(out, HasconfigMatch(alias))
		default:
			// MatchGitdir and any unknown kind fall back to the project gitdir.
			out = append(out, GitdirMatch(cloneName))
		}
	}
	return out
}
