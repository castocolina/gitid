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
	if strings.ContainsAny(identityFile, "\n\r\x00") {
		return &ValidationError{Field: "identityFile", Message: "IdentityFile contains control characters"}
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
		if len(host.Patterns) == 1 && host.Patterns[0].String() == "*" {
			continue
		}
		if len(host.Patterns) == 0 {
			continue
		}
		positive := false
		negative := false
		for _, pat := range host.Patterns {
			s := pat.String()
			negated := strings.HasPrefix(s, "!")
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
		if positive && !negative {
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
