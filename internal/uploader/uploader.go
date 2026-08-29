// Package uploader detects provider CLIs and manages SSH public-key registrations.
package uploader

import (
	"errors"
	"fmt"
	"strings"

	"golang.org/x/crypto/ssh"
)

const (
	githubMainDomain = "github.com"
	gitlabMainDomain = "gitlab.com"
)

// Deps holds uploader external-effect seams.
type Deps struct {
	LookPath func(name string) (string, error)
	RunCmd   func(name string, args ...string) (stdout string, exitCode int, err error)
	// ReadFile reads candidate public-key material. TestUploaderDepsEveryFieldIsWired
	// verifies that buildUploaderDeps supplies this seam in the real constructor.
	ReadFile func(path string) ([]byte, error)
}

// Tool identifies a hosted-git CLI.
type Tool int

const (
	// ToolGH is the GitHub CLI.
	ToolGH Tool = iota
	// ToolGLab is the GitLab CLI.
	ToolGLab
)

// AuthStatus describes a provider CLI authentication state.
type AuthStatus int

const (
	// AuthAuthenticated means the provider CLI is authenticated.
	AuthAuthenticated AuthStatus = iota
	// AuthNotLoggedIn means the provider CLI is available but unauthenticated.
	AuthNotLoggedIn
	// AuthToolNotFound means the provider CLI is absent.
	AuthToolNotFound
)

const (
	// KeyAuthentication is GitHub's authentication registration type.
	KeyAuthentication = "authentication"
	// KeySigning is GitHub's signing registration type.
	KeySigning = "signing"
	// GLabKeyTypeAuthAndSigning is GitLab's documented combined registration type.
	// D-12 closed the old conservative auth-only choice because it lost signing;
	// older glab versions fail into the manual fallback.
	GLabKeyTypeAuthAndSigning = "auth_and_signing"
)

// Registration identifies the provider registration a public key serves.
type Registration int

const (
	// RegistrationAuthentication is an SSH authentication registration.
	RegistrationAuthentication Registration = iota
	// RegistrationSigning is an SSH commit-signing registration.
	RegistrationSigning
	// RegistrationCombined is GitLab's combined auth-and-signing registration.
	RegistrationCombined
)

// KeyTypeFor returns the provider wire value for r.
func (r Registration) KeyTypeFor(tool Tool) (string, error) {
	switch tool {
	case ToolGH:
		switch r {
		case RegistrationAuthentication:
			return KeyAuthentication, nil
		case RegistrationSigning:
			return KeySigning, nil
		}
	case ToolGLab:
		if r == RegistrationCombined {
			return GLabKeyTypeAuthAndSigning, nil
		}
	}
	return "", fmt.Errorf("uploader: registration %d is not supported by %s", r, toolName(tool))
}

// Outcome describes one registration attempt.
type Outcome int

const (
	// OutcomeUploaded means the provider accepted the registration.
	OutcomeUploaded Outcome = iota
	// OutcomeAlreadyPresent means the provider already registered this key.
	OutcomeAlreadyPresent
	// OutcomeFailed means the registration was not completed.
	OutcomeFailed
)

// RegistrationRequest preserves a title per registration. A batch-wide title cannot
// express Phase 9's distinct e2e-purpose titles required by ONESHOT policy.
type RegistrationRequest struct {
	Registration Registration
	Title        string
}

// RegistrationResult reports the result for one requested registration.
type RegistrationResult struct {
	Registration Registration
	Title        string
	Command      string
	Outcome      Outcome
	Output       string
	Err          error
}

// ProviderForHostname returns the supported provider and its canonical web host.
func ProviderForHostname(hostname string) (provider, canonicalHost string) {
	host := normalizeHostname(hostname)
	if isMainDomainOrSubdomain(host, githubMainDomain) {
		return "github", githubMainDomain
	}
	if isMainDomainOrSubdomain(host, gitlabMainDomain) {
		return "gitlab", gitlabMainDomain
	}
	return "", ""
}

func normalizeHostname(hostname string) string {
	host := strings.TrimSuffix(strings.ToLower(strings.TrimSpace(hostname)), ".")
	if idx := strings.LastIndex(host, ":"); idx >= 0 && isAllDigits(host[idx+1:]) {
		return host[:idx]
	}
	return host
}

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

func isMainDomainOrSubdomain(host, mainDomain string) bool {
	return host == mainDomain || strings.HasSuffix(host, "."+mainDomain)
}

// Detect selects the first available authenticated provider CLI.
func Detect(deps Deps) (tool Tool, toolPath string, status AuthStatus) {
	for _, name := range []string{"gh", "glab"} {
		p, err := deps.LookPath(name)
		if err != nil {
			continue
		}
		_, code, _ := deps.RunCmd(p, "auth", "status")
		if code == 0 {
			return toolForName(name), p, AuthAuthenticated
		}
		return toolForName(name), p, AuthNotLoggedIn
	}
	return 0, "", AuthToolNotFound
}

// DetectFor resolves only the CLI corresponding to provider.
func DetectFor(provider string, deps Deps) (tool Tool, toolPath string, status AuthStatus) {
	name := map[string]string{"github": "gh", "gitlab": "glab"}[provider]
	if name == "" {
		return 0, "", AuthToolNotFound
	}
	p, err := deps.LookPath(name)
	if err != nil {
		return 0, "", AuthToolNotFound
	}
	return toolForName(name), p, AuthNotLoggedIn
}

// AuthCheck reports toolPath's authentication status for canonicalHost.
func AuthCheck(toolPath string, deps Deps, canonicalHost string) AuthStatus {
	_, code, _ := deps.RunCmd(toolPath, "auth", "status", "--hostname", canonicalHost)
	if code == 0 {
		return AuthAuthenticated
	}
	return AuthNotLoggedIn
}

// UploadKey validates and uploads one public-key registration.
func UploadKey(tool Tool, toolPath, pubPath string, req RegistrationRequest, deps Deps) RegistrationResult {
	result := RegistrationResult{Registration: req.Registration, Title: req.Title, Outcome: OutcomeFailed}
	if err := requirePublicKey(pubPath, deps); err != nil {
		result.Err = err
		return result
	}
	keyType, err := req.Registration.KeyTypeFor(tool)
	if err != nil {
		result.Err = err
		return result
	}
	result.Command = CommandPreview(tool, toolPath, pubPath, req.Title, keyType)
	args, err := buildArgs(tool, pubPath, req.Title, keyType)
	if err != nil {
		result.Err = err
		return result
	}
	out, code, runErr := deps.RunCmd(toolPath, args...)
	result.Output = trimOutput(out)
	if runErr != nil || code != 0 {
		result.Err = fmt.Errorf("uploader: %s upload failed (exit %d): %w", toolName(tool), code, wrapRunErr(runErr))
		return result
	}
	result.Outcome = OutcomeUploaded
	return result
}

// UploadKeys attempts every request even after a failure. This resolves the
// 09-UI-SPEC unresolved row as continue-on-failure and never derives, defaults, or
// overrides a title.
func UploadKeys(tool Tool, toolPath, pubPath string, reqs []RegistrationRequest, deps Deps) []RegistrationResult {
	results := make([]RegistrationResult, 0, len(reqs))
	for _, req := range reqs {
		results = append(results, UploadKey(tool, toolPath, pubPath, req, deps))
	}
	return results
}

// RegistrationRequestsWithTitle creates the uniform D-07 product requests while
// retaining distinct per-registration titles for the Phase 9 e2e caller.
func RegistrationRequestsWithTitle(title string, regs ...Registration) []RegistrationRequest {
	reqs := make([]RegistrationRequest, len(regs))
	for i, reg := range regs {
		reqs[i] = RegistrationRequest{Registration: reg, Title: title}
	}
	return reqs
}

// requirePublicKey enforces ASVS V5 by content, not only the .pub suffix. It
// promotes the old copy.go convention after review found a renamed private key
// bypassed it, using the authorized-key parser already used by keygen/keyscan.
func requirePublicKey(pubPath string, deps Deps) error {
	if !strings.HasSuffix(pubPath, ".pub") {
		return fmt.Errorf("uploader: refusing non-public-key path %q", pubPath)
	}
	if deps.ReadFile == nil {
		return errors.New("uploader: public-key reader is not configured")
	}
	data, err := deps.ReadFile(pubPath)
	if err != nil {
		return fmt.Errorf("uploader: reading public key %q: %w", pubPath, err)
	}
	if strings.Contains(string(data), "-----BEGIN") && strings.Contains(string(data), "PRIVATE KEY-----") {
		return fmt.Errorf("uploader: refusing private key content at %q", pubPath)
	}
	if _, _, _, _, err := ssh.ParseAuthorizedKey(data); err != nil {
		return fmt.Errorf("uploader: invalid public key content at %q: %w", pubPath, err)
	}
	return nil
}

// KeyTitle returns the frozen UploadKeyTitleFmt twin without importing tuikit.
func KeyTitle(identityName, machineHostname string) string {
	machine := strings.TrimSpace(machineHostname)
	if idx := strings.Index(machine, "."); idx >= 0 {
		machine = machine[:idx]
	}
	if machine == "" {
		machine = "unknown-host"
	}
	return fmt.Sprintf("gitid: %s @ %s", identityName, machine)
}

// TitleMatchesThisMachine permits only this machine's exact D-07 title.
func TitleMatchesThisMachine(title, identityName, machineHostname string) bool {
	return title == KeyTitle(identityName, machineHostname)
}

// CommandPreview renders exactly the argv that UploadKey and UploadKeys run.
func CommandPreview(tool Tool, toolPath, pubPath, title, keyType string) string {
	args, err := buildArgs(tool, pubPath, title, keyType)
	if err != nil {
		return fmt.Sprintf("(preview unavailable: %s)", err)
	}
	return strings.Join(append([]string{toolPath}, args...), " ")
}

func buildArgs(tool Tool, pubPath, title, keyType string) ([]string, error) {
	switch tool {
	case ToolGH:
		return []string{"ssh-key", "add", pubPath, "--title", title, "--type", keyType}, nil
	case ToolGLab:
		return []string{"ssh-key", "add", pubPath, "-t", title, "--usage-type", keyType}, nil
	default:
		return nil, fmt.Errorf("uploader: unknown tool %d", tool)
	}
}

func toolForName(name string) Tool {
	if name == "glab" {
		return ToolGLab
	}
	return ToolGH
}

func toolName(t Tool) string {
	if t == ToolGH {
		return "gh"
	}
	if t == ToolGLab {
		return "glab"
	}
	return fmt.Sprintf("tool(%d)", t)
}

// ToolName returns the CLI name for t.
func ToolName(t Tool) string { return toolName(t) }

func trimOutput(s string) string { return strings.TrimRight(s, "\n") }

// TrimOutput removes trailing newlines from CLI output.
func TrimOutput(s string) string { return trimOutput(s) }

func wrapRunErr(runErr error) error {
	if runErr != nil {
		return runErr
	}
	return errors.New("non-zero exit")
}
