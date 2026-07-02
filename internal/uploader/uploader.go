// Package uploader detects the presence of gh or glab on PATH, checks
// authentication status, and uploads SSH public keys using the detected tool.
// All external effects (LookPath, exec) are injected via Deps so this package
// is testable without real binaries on PATH.
//
// Design constraints (AUTOUP-01):
//   - Detect-then-prompt only: this package never drives an interactive login,
//     never auto-uploads, and never gates create or the test loop. Callers
//     decide per-key with explicit confirmation (D-11, D-12).
//   - All subprocess invocations use explicit arg slices (no sh -c) and go
//     through Deps.RunCmd so unit tests can record calls without a real binary.
//   - The live RunCmd closure (wrapping exec.Command + *exec.ExitError) is NOT
//     defined here. It is wired in Plan 06 in two places:
//     tui/deps.go buildTUIUploaderDeps (TUI surface) and
//     cmd/gitid/copy.go buildUploaderDeps (CLI surface). (REVIEWS.md #11)
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
	// For gh: --type authentication
	// For glab: pass KeyGLabAuth ("auth") via --usage-type (see GLabKeyTypeForAuth).
	KeyAuthentication = "authentication"
	// KeySigning is the key type for SSH commit-signing keys.
	// For gh: --type signing
	// For glab: pass KeyGLabAuth ("auth") via --usage-type; glab does not have
	// a separate signing-only value in all versions — see GLabKeyTypeForAuth.
	KeySigning = "signing"

	// GLabKeyTypeForAuth is the --usage-type value for glab ssh-key add that
	// covers SSH authentication (and, on GitLab.com, signing as well).
	//
	// Open Question A2 (RESEARCH.md §Open Questions): the canonical value for
	// "auth + signing" may be "auth_and_signing" on recent glab versions. We
	// use "auth" as the conservative documented fallback because:
	//   1. glab is not available in the build environment to confirm --help output.
	//   2. "auth" is listed in the docs.gitlab.com/cli/ssh-key/add/ reference.
	//   3. If "auth_and_signing" is required for signing, callers can pass it
	//      explicitly; the arg slice accepts whatever keyType string is provided.
	// Callers that need to upload a signing key separately should pass
	// "auth_and_signing" (or "signing" on future glab versions) as keyType.
	GLabKeyTypeForAuth = "auth"
)

// Detect scans PATH for gh then glab (deterministic order: gh is preferred),
// probes auth status for the first tool found, and returns the tool identifier,
// its resolved path, and the auth status.
//
// Return values when neither tool is found: (0, "", AuthToolNotFound).
// The tool constant (first return) is meaningful only when status != AuthToolNotFound.
func Detect(deps Deps) (tool Tool, toolPath string, status AuthStatus) {
	for _, name := range []string{"gh", "glab"} {
		p, err := deps.LookPath(name)
		if err != nil {
			continue
		}
		// Probe authentication: "gh auth status" / "glab auth status"
		// Exit 0 => authenticated; any non-zero exit => not logged in.
		_, code, _ := deps.RunCmd(p, "auth", "status")
		t := toolForName(name)
		if code == 0 {
			return t, p, AuthAuthenticated
		}
		return t, p, AuthNotLoggedIn
	}
	return 0, "", AuthToolNotFound
}

// AuthCheck probes the authentication status of toolPath by running
// "<toolPath> auth status" and returning the corresponding AuthStatus.
// It does not check whether the tool exists on PATH; callers that need
// both detection and auth should use Detect.
func AuthCheck(toolPath string, deps Deps) AuthStatus {
	_, code, _ := deps.RunCmd(toolPath, "auth", "status")
	if code == 0 {
		return AuthAuthenticated
	}
	return AuthNotLoggedIn
}

// UploadKey uploads the public key at pubPath to the hosted-git platform
// using the tool at toolPath. keyType controls the upload role:
//   - For gh: pass KeyAuthentication ("authentication") or KeySigning ("signing")
//   - For glab: pass GLabKeyTypeForAuth ("auth") or "auth_and_signing"
//
// The arg slice passed to RunCmd is identical to the preview returned by
// CommandPreview (shown command == run command, per UI-SPEC §4a).
//
// NEVER uses shell expansion: args are always passed as an explicit slice to
// Deps.RunCmd. The live RunCmd closure (in Plan 06) uses exec.Command with
// the same arg slice — never "sh -c". (T-05.7-04-01 mitigate)
//
// Only the .pub path is ever accepted; passing a private key path is a caller
// error. (T-05.7-04-02 mitigate — enforced by convention and test fixtures)
//
// On error, the captured output is included in the returned string so callers
// can display a manual fallback (D-11 / D-12).
func UploadKey(tool Tool, toolPath, pubPath, title, keyType string, deps Deps) (string, error) {
	args, err := buildArgs(tool, pubPath, title, keyType)
	if err != nil {
		return "", err
	}
	out, code, runErr := deps.RunCmd(toolPath, args...)
	trimmed := trimOutput(out)
	if runErr != nil || code != 0 {
		return trimmed, fmt.Errorf("uploader: %s upload failed (exit %d): %w",
			toolName(tool), code, wrapRunErr(runErr))
	}
	return trimmed, nil
}

// CommandPreview returns the full command string a user could run manually
// to upload a key. The args used internally by UploadKey are identical to
// those encoded in this preview (shown command == run command, UI-SPEC §4a).
//
// toolPath is the resolved binary path (e.g. "/usr/local/bin/gh").
func CommandPreview(tool Tool, toolPath, pubPath, title, keyType string) string {
	args, err := buildArgs(tool, pubPath, title, keyType)
	if err != nil {
		return fmt.Sprintf("(preview unavailable: %s)", err)
	}
	parts := append([]string{toolPath}, args...)
	return strings.Join(parts, " ")
}

// buildArgs constructs the exact arg slice for gh or glab ssh-key add.
// The slice is used both by UploadKey (execution) and CommandPreview (display),
// ensuring shown command == run command (UI-SPEC §4a).
//
// gh:   ssh-key add <pubPath> --title <title> --type <keyType>
// glab: ssh-key add <pubPath> -t <title> --usage-type <keyType>
func buildArgs(tool Tool, pubPath, title, keyType string) ([]string, error) {
	switch tool {
	case ToolGH:
		// gh ssh-key add <key-file> --title "gitid: <name>" --type authentication|signing
		// Documented at: https://cli.github.com/manual/gh_ssh-key_add
		return []string{"ssh-key", "add", pubPath, "--title", title, "--type", keyType}, nil
	case ToolGLab:
		// glab ssh-key add <key-file> -t "gitid: <name>" --usage-type auth
		// Documented at: https://docs.gitlab.com/cli/ssh-key/add/
		// Note: glab uses -t (short flag) for title; --usage-type for role.
		// See GLabKeyTypeForAuth for the open question on "auth" vs "auth_and_signing".
		return []string{"ssh-key", "add", pubPath, "-t", title, "--usage-type", keyType}, nil
	default:
		return nil, fmt.Errorf("uploader: unknown tool %d", tool)
	}
}

// toolForName maps a binary name to its Tool constant.
func toolForName(name string) Tool {
	if name == "glab" {
		return ToolGLab
	}
	return ToolGH
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

// trimOutput trims trailing newlines from exec output.
func trimOutput(s string) string {
	return strings.TrimRight(s, "\n")
}

// TrimOutput is the exported form for use by callers that process RunCmd output.
func TrimOutput(s string) string {
	return trimOutput(s)
}

// wrapRunErr returns a sentinel error when runErr is nil but the exit code was
// non-zero, so fmt.Errorf %w always has a non-nil target.
func wrapRunErr(runErr error) error {
	if runErr != nil {
		return runErr
	}
	return errors.New("non-zero exit")
}
