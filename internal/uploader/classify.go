package uploader

import (
	"regexp"
	"strings"
)

// FailureKind categorizes provider failures for the render layer.
type FailureKind int

const (
	// FailureNone means no failure needs remediation.
	FailureNone FailureKind = iota
	// FailureScopeAuth means the authentication-key scope is missing.
	FailureScopeAuth
	// FailureScopeSigning means the signing-key scope is missing.
	FailureScopeSigning
	// FailureCrossAccountConflict means GitLab reports the key belongs elsewhere.
	FailureCrossAccountConflict
	// FailureNotAuthenticated means the provider CLI session is unavailable.
	FailureNotAuthenticated
	// FailureUnknown means no stable provider failure kind matched.
	FailureUnknown
)

var (
	ghTokenPattern   = regexp.MustCompile(`\bgh[pousr]_[A-Za-z0-9]{20,}\b`)
	glabTokenPattern = regexp.MustCompile(`\bglpat-[A-Za-z0-9_-]{20,}\b`)
)

// ClassifyUploadFailure maps a failed registration to a stable remediation kind.
func ClassifyUploadFailure(tool Tool, reg Registration, res RegistrationResult) FailureKind {
	if res.Outcome != OutcomeFailed {
		return FailureNone
	}
	output := strings.ToLower(res.Output)
	if strings.Contains(output, "admin:public_key") {
		return FailureScopeAuth
	}
	if strings.Contains(output, "admin:ssh_signing_key") {
		return FailureScopeSigning
	}
	if tool == ToolGH && strings.Contains(output, "admin:") {
		if reg == RegistrationSigning {
			return FailureScopeSigning
		}
		return FailureScopeAuth
	}
	if tool == ToolGLab && strings.Contains(output, "already taken") {
		// GitLab fingerprints are globally unique: this is another account's key,
		// never a successful already-present registration.
		return FailureCrossAccountConflict
	}
	if strings.Contains(output, "not logged in") || strings.Contains(output, "not authenticated") {
		return FailureNotAuthenticated
	}
	return FailureUnknown
}

// ClassifyGHDuplicate reports a zero-exit GitHub duplicate response.
func ClassifyGHDuplicate(res RegistrationResult) bool {
	return res.Err == nil && strings.Contains(strings.ToLower(res.Output), "already exists")
}

// RedactCLIOutput returns the first meaningful raw line without token-shaped
// substrings or the supplied home path. This is defence in depth for future CLI
// versions rather than an assumption that current upload commands never echo tokens.
func RedactCLIOutput(raw string, homeDir string, maxWidth int) string {
	line := ""
	for _, candidate := range strings.Split(raw, "\n") {
		if candidate = strings.TrimSpace(candidate); candidate != "" {
			line = candidate
			break
		}
	}
	line = ghTokenPattern.ReplaceAllString(line, "[redacted]")
	line = glabTokenPattern.ReplaceAllString(line, "[redacted]")
	if homeDir != "" {
		line = strings.ReplaceAll(line, homeDir, "~")
	}
	if maxWidth < 0 {
		return ""
	}
	runes := []rune(line)
	if len(runes) > maxWidth {
		return string(runes[:maxWidth])
	}
	return line
}
