// Package uploader detects the presence of gh or glab on PATH, checks
// authentication status, and uploads SSH public keys using the detected tool.
// All external effects (LookPath, exec) are injected via Deps so this package
// is testable without real binaries on PATH.
package uploader

import (
	"errors"
	"fmt"
	"strings"
)

// Deps holds all external effects. Build live in tui/deps.go;
// pass fakes in tests. Every function field must be non-nil (wiring guard in
// tui/wiring_test.go TestBuildTUIDepsNilGuard_Phase57).
type Deps struct {
	// LookPath resolves a binary name to its full path. Wire to: exec.LookPath
	LookPath func(name string) (string, error)
	// RunCmd runs name with args, returning stdout, exit code, and any error.
	RunCmd func(name string, args ...string) (stdout string, exitCode int, err error)
}

// Tool identifies which hosted-git CLI tool was detected on PATH.
type Tool int

const (
	// ToolGH represents the GitHub CLI (gh).
	ToolGH Tool = iota
	// ToolGLab represents the GitLab CLI (glab).
	ToolGLab
)

// AuthStatus describes the authentication state of the detected tool.
type AuthStatus int

const (
	// AuthAuthenticated means the tool is present and the user is logged in.
	AuthAuthenticated AuthStatus = iota
	// AuthNotLoggedIn means the tool is present but the user is not logged in.
	AuthNotLoggedIn
	// AuthToolNotFound means neither gh nor glab was found on PATH.
	AuthToolNotFound
)

// Key type constants for the --type flag (gh) and --usage-type (glab).
const (
	// KeyAuthentication is the key type for SSH authentication keys.
	KeyAuthentication = "authentication"
	// KeySigning is the key type for SSH commit-signing keys.
	KeySigning = "signing"
)

// Detect scans PATH for gh then glab, probes auth status, and returns the first
// found tool together with its path and authentication status.
//
// RED stub: returns zero values + AuthToolNotFound. Plan 04 (05.7-04) implements.
func Detect(_ Deps) (tool Tool, toolPath string, status AuthStatus) {
	return 0, "", AuthToolNotFound
}

// UploadKey uploads pubPath to tool using toolPath. keyType should be
// KeyAuthentication or KeySigning for gh; "auth" for glab.
// NEVER uses shell expansion — arg-slice only (G204-clean).
//
// RED stub: returns empty + sentinel. Plan 04 (05.7-04) implements.
func UploadKey(_, _, _, _, _ string, _ Deps) (string, error) {
	return "", errors.New("uploader: not implemented")
}

// CommandPreview returns the shell command string a user would run manually
// to upload a key, without actually executing anything.
//
// RED stub: returns empty string. Plan 04 (05.7-04) implements.
func CommandPreview(_, _, _, _, _ string) string {
	return ""
}

// toolName returns the human-readable name for a Tool value.
func toolName(t Tool) string {
	switch t {
	case ToolGH:
		return "gh"
	case ToolGLab:
		return "glab"
	default:
		return fmt.Sprintf("tool(%d)", t)
	}
}

// ToolName is the exported form of toolName for use in tui/copy.go view rendering.
func ToolName(t Tool) string {
	return toolName(t)
}

// trimOutput trims trailing newline from exec output.
func trimOutput(s string) string {
	return strings.TrimRight(s, "\n")
}

// TrimOutput is the exported form for use by callers that process RunCmd output.
func TrimOutput(s string) string {
	return trimOutput(s)
}
