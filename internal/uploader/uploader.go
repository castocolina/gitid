// Package uploader detects the presence of gh or glab on PATH, checks
// authentication status, and uploads SSH public keys using the detected tool.
// All external effects (LookPath, exec) are injected via Deps so this package
// is testable without real binaries on PATH.
//
// Design constraints (AUTOUP-01):
//   - All subprocess invocations use explicit arg slices (no sh -c) and go
//     through Deps.RunCmd so unit tests can record calls without a real binary.
//   - The live RunCmd closure (wrapping exec.Command + *exec.ExitError) is
//     wired in cmd/gitid/wiring.go's buildUploaderDeps — the ONE wiring site
//     (the earlier "tui/deps.go" / "cmd/gitid/copy.go" split named above was
//     the pre-Phase-9-tracer POC layout; both are archived).
//   - Phase 9 (UP-02/UP-03, D-01/D-02) supersedes the former "never
//     auto-uploads" constraint this comment used to state: when the detected
//     tool is present, authenticated for the identity's canonical host, and
//     the user has not opted out, the create wizard drives UploadKey
//     autonomously — announce-then-run, never a per-key interactive prompt.
//     Detection and auth-checking stay exactly as conservative as before;
//     only the "must a human explicitly trigger every upload" rule changed.
package uploader

import (
	"errors"
	"fmt"
	"strings"
)

// Canonical main-domain hosts D-13 gates autonomous upload to. Declared as
// constants (not inlined into ProviderForHostname) so the equality test and
// the dot-anchored suffix test can never independently drift apart.
const (
	githubMainDomain = "github.com"
	gitlabMainDomain = "gitlab.com"
)

// ProviderForHostname implements D-13's "v1.0 autonomous upload is gated to
// github.com / gitlab.com hosts only" rule with HOST-BOUNDARY matching —
// never an unanchored substring test. A prior draft of this function used
// strings.Contains(host, "github"), which the cross-AI review (R2) found
// would misclassify github.example.com, notgithub.com, mygithub.com, and
// github.com.evil.net as GitHub — pointing an autonomous key upload at the
// wrong (or an attacker-controlled) provider.
//
// Decision-conflict resolution recorded here (R2): D-11's "lowercase
// substring match, same convention as upload.Instructions" describes the
// PROVIDER-KEY convention — upload.Instructions receives a provider token
// ("github"), never a hostname, and DetectFor (below) is where that
// never-cross-route rule lives. This function implements the SEPARATE,
// stricter D-13 HOSTNAME gate: exact match on the canonical domain, or a
// proper dot-anchored subdomain of it.
//
// Returns the provider key ("github"/"gitlab") and the CANONICAL host
// ("github.com"/"gitlab.com") — never an echo of the input. Callers (R18/the
// review's canonical-host regression) MUST pass this canonical host, not the
// raw hostname, to AuthCheck: gh/glab track authentication per canonical web
// domain, never per the SSH endpoint an identity happens to connect through,
// and this project's own alt-SSH recipe (ssh.github.com, port 443) makes
// that divergence the COMMON case for real identities, not an edge case.
//
// Anything that is neither of the two main domains nor a subdomain of one —
// including a host that merely CONTAINS a provider's name — returns ("", "")
// so the caller falls back to the manual instructions path.
func ProviderForHostname(hostname string) (provider, canonicalHost string) {
	host := normalizeHostname(hostname)
	if host == "" {
		return "", ""
	}
	if isMainDomainOrSubdomain(host, githubMainDomain) {
		return "github", githubMainDomain
	}
	if isMainDomainOrSubdomain(host, gitlabMainDomain) {
		return "gitlab", gitlabMainDomain
	}
	return "", ""
}

// normalizeHostname lowercases hostname, trims surrounding whitespace, drops
// a trailing dot, and drops a trailing ":port" suffix — so
// "GitHub.com", "github.com.", and "github.com:443" all normalize to the
// same comparable value.
func normalizeHostname(hostname string) string {
	host := strings.ToLower(strings.TrimSpace(hostname))
	host = strings.TrimSuffix(host, ".")
	if idx := strings.LastIndex(host, ":"); idx >= 0 {
		// Only strip a trailing :port — never touch a bare IPv6 literal
		// (this package never receives one; provider hostnames are always
		// DNS names), and never strip when nothing follows the colon.
		if port := host[idx+1:]; port != "" && isAllDigits(port) {
			host = host[:idx]
		}
	}
	return host
}

// isAllDigits reports whether s is a non-empty run of ASCII digits.
func isAllDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// isMainDomainOrSubdomain reports whether host equals mainDomain or ends
// with "."+mainDomain — the dot-anchored suffix test that keeps
// "github.example.com" and "notgithub.com" from matching "github.com".
func isMainDomainOrSubdomain(host, mainDomain string) bool {
	if host == mainDomain {
		return true
	}
	return strings.HasSuffix(host, "."+mainDomain)
}

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
// Deprecated: Detect is the D-11 first-found router this package's Phase-9
// wiring no longer calls — a caller that knows the identity's PROVIDER (from
// ProviderForHostname) must route through DetectFor instead, which never
// cross-routes a GitHub identity's key to an authenticated glab or vice
// versa. Kept only until Phase 9's tracer wave lands its own callers; slated
// for removal once nothing references it.
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

// DetectFor resolves the ONE tool that matches provider ("github" or
// "gitlab", as ProviderForHostname returns) and looks up ONLY that tool —
// D-11's never-cross-route rule enforced structurally rather than by
// convention. A GitHub identity never probes glab even when glab is present
// and authenticated and gh is not; a GitLab identity never probes gh.
//
// It does NOT probe auth status — callers call AuthCheck separately with the
// CANONICAL host ProviderForHostname returned, never a raw input host (R18).
// The returned status is provisional: AuthAuthenticated is never returned
// here, because auth is not checked; a successful LookPath returns
// AuthNotLoggedIn as a placeholder the caller immediately refines via
// AuthCheck. An unknown provider, or a provider whose tool is absent from
// PATH, returns (0, "", AuthToolNotFound) — and for an unknown provider key,
// LookPath is never called at all.
func DetectFor(provider string, deps Deps) (tool Tool, toolPath string, status AuthStatus) {
	var name string
	switch provider {
	case "github":
		name = "gh"
	case "gitlab":
		name = "glab"
	default:
		return 0, "", AuthToolNotFound
	}
	p, err := deps.LookPath(name)
	if err != nil {
		return 0, "", AuthToolNotFound
	}
	return toolForName(name), p, AuthNotLoggedIn
}

// AuthCheck probes the authentication status of toolPath by running
// "<toolPath> auth status --hostname <canonicalHost>" and returning the
// corresponding AuthStatus. D-14 is explicit that a bare "auth status"
// checks ALL hosts — wrong in both directions — so canonicalHost is
// required, never optional. canonicalHost MUST be the value
// ProviderForHostname's second return produced, never the raw wizard-
// supplied hostname (R18): gh/glab track authentication per canonical web
// domain, not per SSH endpoint, and this project's alt-SSH recipe
// (ssh.github.com, port 443) makes that divergence the common case for real
// identities. It does not check whether the tool exists on PATH; callers
// that need both detection and auth should use DetectFor first.
func AuthCheck(toolPath string, deps Deps, canonicalHost string) AuthStatus {
	_, code, _ := deps.RunCmd(toolPath, "auth", "status", "--hostname", canonicalHost)
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
