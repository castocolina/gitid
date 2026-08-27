package sshconfig

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	ssh_config "github.com/kevinburke/ssh_config"
)

// ValidationError is a structured, field-keyed error returned by
// ValidateHostBlock. Callers render it inline on the offending form field.
type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string { return e.Message }

// validateToken reports whether s is a safe, non-empty SSH config token: no
// whitespace, control characters, wildcard/negation metacharacters, commas, or
// directive-like separators. These checks apply to Host aliases and hostnames
// before they are interpolated into a rendered block.
func validateToken(s string) error {
	if strings.TrimSpace(s) == "" {
		return fmt.Errorf("cannot be empty")
	}
	for _, r := range s {
		switch {
		case r <= ' ' || r == 0x7f:
			return fmt.Errorf("contains whitespace or control character")
		case r == '*', r == '?':
			return fmt.Errorf("contains wildcard %q", r)
		case r == '!':
			return fmt.Errorf("contains negation %q", r)
		case r == ',':
			return fmt.Errorf("contains comma %q", r)
		case r == '\n' || r == '\r':
			return fmt.Errorf("contains line break")
		}
	}
	return nil
}

// ValidateHostBlock validates the four SSH form values before they are
// interpolated into an OpenSSH Host block. It returns a field-keyed
// ValidationError so the UI can render the failure on the offending control.
func ValidateHostBlock(alias, hostname, portStr, identityFile string) error {
	if err := validateToken(alias); err != nil {
		return &ValidationError{Field: "alias", Message: fmt.Sprintf("SSH Host invalid: %v", err)}
	}
	if err := validateToken(hostname); err != nil {
		return &ValidationError{Field: "hostname", Message: fmt.Sprintf("Real hostname invalid: %v", err)}
	}
	port, err := strconv.Atoi(strings.TrimSpace(portStr))
	if err != nil {
		return &ValidationError{Field: "port", Message: "Port must be a number"}
	}
	if port < 1 || port > 65535 {
		return &ValidationError{Field: "port", Message: fmt.Sprintf("Port %d is outside 1-65535", port)}
	}
	if strings.TrimSpace(identityFile) == "" {
		return &ValidationError{Field: "identityFile", Message: "IdentityFile cannot be empty"}
	}
	// CR-11: Reject every character that is unsafe in an unquoted OpenSSH
	// IdentityFile token. The renderer interpolates this value without quoting,
	// so any such character would produce a malformed or semantically different
	// SSH directive.
	//
	// Rejected: any ASCII/Unicode whitespace or control character, quotes,
	// backslash (ambiguous escaping), hash (comment separator), equals
	// (directive key=value separator), and NUL/DEL.
	for i, r := range identityFile {
		switch {
		case r == 0: // NUL
			return &ValidationError{Field: "identityFile", Message: "IdentityFile contains NUL character"}
		case r == '\n' || r == '\r':
			return &ValidationError{Field: "identityFile", Message: "IdentityFile contains line break"}
		case r < ' ' || r == 0x7f: // other control characters and DEL
			return &ValidationError{Field: "identityFile", Message: fmt.Sprintf("IdentityFile contains control character at byte %d", i)}
		case r == ' ' || r == '\t' || r == '\f' || r == '\v':
			return &ValidationError{Field: "identityFile", Message: "IdentityFile contains whitespace — unquoted OpenSSH tokens may not contain spaces or tabs"}
		case r > 0x7e && r <= 0x9f: // C1 control characters
			return &ValidationError{Field: "identityFile", Message: fmt.Sprintf("IdentityFile contains Unicode control character U+%04X", r)}
		case r == '"' || r == '\'':
			return &ValidationError{Field: "identityFile", Message: "IdentityFile contains a quote character — unquoted tokens may not contain quotes"}
		case r == '\\':
			return &ValidationError{Field: "identityFile", Message: "IdentityFile contains a backslash — ambiguous escaping in unquoted tokens is not supported"}
		case r == '#':
			return &ValidationError{Field: "identityFile", Message: "IdentityFile contains '#' — interpreted as a comment separator in SSH config"}
		case r == '=':
			return &ValidationError{Field: "identityFile", Message: "IdentityFile contains '=' — interpreted as a key-value separator in SSH config"}
		}
	}
	return nil
}

// HostMatch reports whether an existing Host pattern matches candidate using
// OpenSSH glob semantics: '*' matches any sequence (including empty), '?' matches
// any single character. Matching is case-insensitive because SSH host names are.
func HostMatch(pattern, candidate string) bool {
	pattern = strings.ToLower(strings.TrimSpace(pattern))
	candidate = strings.ToLower(strings.TrimSpace(candidate))
	if pattern == "*" {
		return true
	}
	return globMatch(pattern, candidate)
}

// globMatch is a simple recursive glob matcher for '*' and '?'.
func globMatch(pattern, s string) bool {
	for len(pattern) > 0 {
		switch pattern[0] {
		case '*':
			// Consume consecutive stars.
			for len(pattern) > 0 && pattern[0] == '*' {
				pattern = pattern[1:]
			}
			if len(pattern) == 0 {
				return true
			}
			for i := 0; i <= len(s); i++ {
				if globMatch(pattern, s[i:]) {
					return true
				}
			}
			return false
		case '?':
			if len(s) == 0 {
				return false
			}
			pattern, s = pattern[1:], s[1:]
		default:
			if len(s) == 0 || pattern[0] != s[0] {
				return false
			}
			pattern, s = pattern[1:], s[1:]
		}
	}
	return len(s) == 0
}

// HostPatternsMatch reports whether a Host stanza whose pattern tokens are
// patterns matches candidate under OpenSSH stanza semantics: the stanza
// matches when at least one non-negated pattern matches the candidate AND no
// negated pattern in the same stanza matches it.
//
// Each token in patterns is taken exactly as returned by pat.String() on the
// kevinburke/ssh_config parser's Pattern type, negation prefix ("!") included.
// The trim of the "!" prefix is performed HERE, never at the call site, so a
// caller that pre-trims would silently turn every negation into an inclusion.
//
// HostLineMatches is the companion that accepts a raw whitespace-separated Host
// line string and calls this function after tokenising it — both functions
// therefore operate on the same negation-carrying token shape, so the rule
// lives in exactly one place.
//
// internal/globalssh's shadow-naming pass is the second caller; adding a third
// negation implementation anywhere is the defect this export exists to prevent
// (06-REVIEWS.md cycle-3 HIGH: the previous revision told shadow.go to call
// the unexported aliasCollides).
func HostPatternsMatch(patterns []string, candidate string) bool {
	if len(patterns) == 0 {
		return false
	}
	positive := false
	negative := false
	for _, tok := range patterns {
		negated := strings.HasPrefix(tok, "!")
		s := tok
		if negated {
			s = strings.TrimPrefix(s, "!")
		}
		if HostMatch(s, candidate) {
			if negated {
				negative = true
			} else {
				positive = true
			}
		}
	}
	return positive && !negative
}

// HostLineMatches applies the same negation-aware stanza rule as
// HostPatternsMatch to the raw whitespace-separated pattern text of a Host
// line — the shape DirectiveHit.HostPattern carries. strings.Fields tokenises
// the line into the same negation-carrying form pat.String() returns, so both
// callers hand HostPatternsMatch identical tokens and the rule is decided in
// one place.
func HostLineMatches(patternLine, candidate string) bool {
	return HostPatternsMatch(strings.Fields(patternLine), candidate)
}

// HostPattern is one pattern from a Host line, annotated with its negation
// state and the file it came from.
type HostPattern struct {
	Pattern string
	Negated bool
	File    string
}

// AliasCollision reports whether candidate is already claimed by an existing
// Host stanza in configPath or any file its Include directives resolve to.
// It follows OpenSSH first-match-wins semantics and resolves negated patterns
// per-stanza: a Host stanza matches candidate only when at least one
// non-negated pattern matches and no negated pattern in the same stanza
// matches. A read, parse, symlink, or cyclic Include error is returned as a
// blocking failure rather than false.
func AliasCollision(configPath, candidate string) (bool, error) {
	return aliasCollides(configPath, candidate, nil)
}

func aliasCollides(configPath, candidate string, seen map[string]bool) (bool, error) {
	if seen == nil {
		seen = make(map[string]bool)
	}
	abs, err := filepath.Abs(configPath)
	if err != nil {
		return false, fmt.Errorf("sshconfig: resolving %s: %w", configPath, err)
	}
	if seen[abs] {
		return false, fmt.Errorf("sshconfig: cyclic Include detected at %s", abs)
	}
	seen[abs] = true

	content, err := os.ReadFile(abs) //nolint:gosec // configPath is a trusted gitid-managed path supplied in-process
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, fmt.Errorf("sshconfig: reading %s: %w", abs, err)
	}

	cfg, err := ssh_config.Decode(strings.NewReader(string(content)))
	if err != nil {
		return false, fmt.Errorf("sshconfig: parsing %s: %w", abs, err)
	}

	for _, host := range cfg.Hosts {
		// Skip the implicit Host * inserted by the parser for an empty file.
		// This guard stays at the call site: folding it into HostPatternsMatch
		// would both change AliasCollision's behavior and blind internal/globalssh
		// to a real `Host *` stanza (resolution_order row 4's dominant shadowing
		// case — HostPatternsMatch([]string{"*"}, ProbeHost) MUST return true).
		if len(host.Patterns) == 1 && host.Patterns[0].String() == "*" {
			continue
		}
		// The zero-patterns guard is similarly a fact about cfg.Hosts' shape,
		// not pattern semantics — it stays here for the same reason.
		if len(host.Patterns) == 0 {
			continue
		}
		// Project []*ssh_config.Pattern to []string, keeping the negation prefix
		// exactly as pat.String() returns it. The trim of "!" belongs inside
		// HostPatternsMatch, never here — pre-trimming silently converts every
		// negation into an inclusion (matcher_extraction call-site rule 2).
		tokens := make([]string, 0, len(host.Patterns))
		for _, pat := range host.Patterns {
			tokens = append(tokens, pat.String())
		}
		if HostPatternsMatch(tokens, candidate) {
			return true, nil
		}
	}

	directives, err := DetectInclude(abs)
	if err != nil {
		return false, fmt.Errorf("sshconfig: detecting includes in %s: %w", abs, err)
	}
	for _, d := range directives {
		if !isAcceptablePathForm(d.Raw) {
			continue
		}
		matches, gerr := filepath.Glob(d.Expanded)
		if gerr != nil {
			return false, fmt.Errorf("sshconfig: globbing %s: %w", d.Expanded, gerr)
		}
		for _, m := range matches {
			info, lerr := os.Lstat(m)
			if lerr == nil && info.Mode()&os.ModeSymlink != 0 {
				return false, fmt.Errorf("sshconfig: symlinked Include %s rejected", m)
			}
			if lerr != nil && !os.IsNotExist(lerr) {
				return false, fmt.Errorf("sshconfig: stat %s: %w", m, lerr)
			}
			collides, cerr := aliasCollides(m, candidate, seen)
			if cerr != nil {
				return false, cerr
			}
			if collides {
				return true, nil
			}
		}
	}
	return false, nil
}
