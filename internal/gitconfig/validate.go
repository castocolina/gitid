package gitconfig

import (
	"fmt"
	"strings"
)

// ValidateDefaultBranch validates a git ref name as would be accepted by
// `git check-ref-format --branch`. A valid default branch name must:
//   - Not be empty
//   - Not contain whitespace or control characters (0x00-0x1f, 0x7f)
//   - Not contain special characters: ~ ^ : ? * [ \
//   - Not contain .. or // or trailing .
//   - Not start with - or .
//   - Not be @ alone or @{anything}
//   - Not end with .lock
//   - Not be all dots (., ..)
//
// The validator is pinned differentially against the real git binary by
// tests in plan 09.6-01 Task 2 (behavior Tests 3b-3c).
func ValidateDefaultBranch(name string) error {
	if name == "" {
		return fmt.Errorf("branch name cannot be empty")
	}

	// Disallow control characters and DEL (0x7f)
	for i, r := range name {
		if r < 0x20 || r == 0x7f {
			return fmt.Errorf("branch name contains invalid character at position %d: U+%04X", i, r)
		}
	}

	// Disallow whitespace
	if strings.ContainsAny(name, " \t\n\r") {
		return fmt.Errorf("branch name contains whitespace")
	}

	// Disallow reserved characters
	if strings.ContainsAny(name, "~^:?*[\\") {
		return fmt.Errorf("branch name contains reserved character (~^:?*[\\)")
	}

	// Disallow .. and //
	if strings.Contains(name, "..") {
		return fmt.Errorf("branch name contains consecutive dots (..)")
	}
	if strings.Contains(name, "//") {
		return fmt.Errorf("branch name contains consecutive slashes (//)")
	}

	// Disallow leading - or .
	if name[0] == '-' {
		return fmt.Errorf("branch name cannot start with '-'")
	}
	if name[0] == '.' {
		return fmt.Errorf("branch name cannot start with '.'")
	}

	// Disallow trailing .
	if name[len(name)-1] == '.' {
		return fmt.Errorf("branch name cannot end with '.'")
	}

	// Disallow .lock suffix
	if strings.HasSuffix(name, ".lock") {
		return fmt.Errorf("branch name cannot end with '.lock'")
	}

	// Disallow @ alone or @{...} patterns
	if name == "@" {
		return fmt.Errorf("branch name cannot be '@'")
	}
	if strings.HasPrefix(name, "@{") {
		return fmt.Errorf("branch name cannot use @{...} syntax")
	}

	// Disallow all-dots names (., .., etc.)
	isAllDots := true
	for _, r := range name {
		if r != '.' {
			isAllDots = false
			break
		}
	}
	if isAllDots {
		return fmt.Errorf("branch name cannot be all dots")
	}

	// Basic validation: component separation by /
	// Each component between slashes must not be empty and must not be all dots
	parts := strings.Split(name, "/")
	for _, part := range parts {
		if part == "" {
			return fmt.Errorf("branch name contains empty path component")
		}
		// Check if component is all dots
		isPartDots := true
		for _, r := range part {
			if r != '.' {
				isPartDots = false
				break
			}
		}
		if isPartDots {
			return fmt.Errorf("branch name contains component that is all dots: %q", part)
		}
	}

	return nil
}
