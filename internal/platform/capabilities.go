package platform

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// AgentStatus is a three-valued classification of the local ssh-agent's
// state — distinct from a single coarse bool so callers can tell "no agent"
// apart from "agent locked/unresponsive" and "agent running with no keys
// loaded" (PLAT-02).
type AgentStatus int

// AgentStatus values, derived from `ssh-add -l` exit codes (see probeAgent).
const (
	// AgentAbsent means no ssh-agent is reachable (SSH_AUTH_SOCK is unset).
	AgentAbsent AgentStatus = iota
	// AgentRunningNoKeys means the agent is reachable but has no identities
	// loaded (ssh-add -l exit code 1).
	AgentRunningNoKeys
	// AgentRunning means the agent is reachable and has at least one
	// identity loaded (ssh-add -l exit code 0).
	AgentRunning
	// AgentLockedOrUnavailable means the agent socket exists but the agent
	// could not be queried (locked, unresponsive, or the probe timed out).
	AgentLockedOrUnavailable
)

// String renders the AgentStatus for debug/list output (D-08).
func (s AgentStatus) String() string {
	switch s {
	case AgentAbsent:
		return "absent"
	case AgentRunningNoKeys:
		return "running-no-keys"
	case AgentRunning:
		return "running"
	case AgentLockedOrUnavailable:
		return "locked-or-unavailable"
	default:
		return "unknown"
	}
}

// FIDOStatus is a three-valued classification of local FIDO2/hardware-key
// support. Absent support is the expected common case and is reported as a
// normal, non-fatal status (T-01-03), never an error.
type FIDOStatus int

// FIDOStatus values.
const (
	// FIDOAbsent means `ssh -Q key` lists no sk- token at all.
	FIDOAbsent FIDOStatus = iota
	// FIDOTokenListedOnly means `ssh -Q key` lists an sk- token but the
	// libfido2/ssh-sk-helper middleware needed to actually use it is not
	// present.
	FIDOTokenListedOnly
	// FIDOUsable means an sk- token is listed AND ssh-sk-helper is present.
	FIDOUsable
)

// String renders the FIDOStatus for debug/list output (D-08).
func (s FIDOStatus) String() string {
	switch s {
	case FIDOAbsent:
		return "absent"
	case FIDOTokenListedOnly:
		return "token-listed-only"
	case FIDOUsable:
		return "usable"
	default:
		return "unknown"
	}
}

// Usable collapses the three-valued FIDOStatus into a plain bool for callers
// (01-02/01-06) that only need "can I generate/use a -sk key right now,"
// without importing the enum's finer semantics.
func (s FIDOStatus) Usable() bool {
	return s == FIDOUsable
}

// KeychainStatus reports macOS Keychain (`UseKeychain` /
// `--apple-use-keychain`) support.
type KeychainStatus int

// KeychainStatus values.
const (
	KeychainUnsupported KeychainStatus = iota
	KeychainSupported
)

// String renders the KeychainStatus for debug/list output (D-08).
func (s KeychainStatus) String() string {
	if s == KeychainSupported {
		return "supported"
	}
	return "unsupported"
}

// Capabilities is the full local-capability probe result: agent/FIDO/
// keychain status, the parsed local SSH version, and the supported key-type
// tokens plus their resolved catalog algorithm names.
type Capabilities struct {
	Agent      AgentStatus
	FIDO       FIDOStatus
	Keychain   KeychainStatus
	SSHVersion SSHVersion
	KeyTypes   []string
	Algorithms []string
}

// Deps holds every external effect the capability probe needs, each as an
// injected function field, so Probe is fully mockable in tests — closing
// the project's documented "injected-seam wiring blindspot" by keeping every
// field non-nil in both the real BuildProbeDeps() wiring and test fakes.
type Deps struct {
	ProbeAgent      func(ctx context.Context) AgentStatus
	ProbeFIDO       func(ctx context.Context) FIDOStatus
	ProbeKeychain   func() KeychainStatus
	ProbeSSHVersion func() (SSHVersion, error)
	ProbeKeyTypes   func() ([]string, error)
}

// Probe runs every injected probe and assembles a Capabilities value. It is
// pure orchestration over deps — no external effect lives here directly — so
// it is trivially testable with fakes.
func Probe(ctx context.Context, deps Deps) (Capabilities, error) {
	sshVersion, err := deps.ProbeSSHVersion()
	if err != nil {
		return Capabilities{}, fmt.Errorf("platform: probing ssh version: %w", err)
	}
	keyTypes, err := deps.ProbeKeyTypes()
	if err != nil {
		return Capabilities{}, fmt.Errorf("platform: probing ssh key types: %w", err)
	}
	return Capabilities{
		Agent:      deps.ProbeAgent(ctx),
		FIDO:       deps.ProbeFIDO(ctx),
		Keychain:   deps.ProbeKeychain(),
		SSHVersion: sshVersion,
		KeyTypes:   keyTypes,
		Algorithms: SupportedAlgorithms(keyTypes),
	}, nil
}

// BuildProbeDeps wires the real, exec.CommandContext-backed probe
// implementations (EXPORTED — capital B — so cmd/gitid and the e2e test in
// 01-06 can call it across the package boundary, mirroring 01-04's exported
// BuildInventoryDeps; an unexported buildProbeDeps would be a Go-visibility
// compile blocker for those callers).
func BuildProbeDeps() Deps {
	return Deps{
		ProbeAgent:      probeAgent,
		ProbeFIDO:       probeFIDO,
		ProbeKeychain:   func() KeychainStatus { return probeKeychain(CurrentOS()) },
		ProbeSSHVersion: ProbeSSHVersion,
		ProbeKeyTypes:   ProbeKeyTypes,
	}
}

// probeKeychain reports whether the given OS supports the macOS Keychain
// integration. It is KeychainSupported only on darwin (reusing
// SupportsUseKeychain) — Linux has no keychain concept to probe, regardless
// of any injected agent state (PLAT-02).
func probeKeychain(os string) KeychainStatus {
	if SupportsUseKeychain(os) {
		return KeychainSupported
	}
	return KeychainUnsupported
}

// probeAgent probes the local ssh-agent via `ssh-add -l`, classifying the
// result by exit code: 0 = running with identities loaded, 1 = running with
// no identities, anything else (including a timeout) = locked/unavailable.
// The probe runs under a bounded exec.CommandContext timeout derived from
// ctx (T-01-03: a hung agent must never block gitid).
func probeAgent(ctx context.Context) AgentStatus {
	if os.Getenv("SSH_AUTH_SOCK") == "" {
		return AgentAbsent
	}

	cctx, cancel := context.WithTimeout(ctx, probeTimeout)
	defer cancel()

	err := exec.CommandContext(cctx, "ssh-add", "-l").Run() // #nosec G204 -- fixed args, no user input
	if err == nil {
		return AgentRunning
	}
	if cctx.Err() != nil {
		// Timed out or was cancelled: the agent socket exists but the agent
		// could not be queried in time — treat as locked/unavailable rather
		// than blocking gitid indefinitely.
		return AgentLockedOrUnavailable
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) && exitErr.ExitCode() == 1 {
		return AgentRunningNoKeys
	}
	// Exit code 2 (cannot connect to agent) or any other non-exit error
	// (e.g. ssh-add not found).
	return AgentLockedOrUnavailable
}

// probeFIDO detects libfido2/FIDO2 hardware-key support: it probes
// `ssh -Q key` for an sk- token (the protocol advertises FIDO2 support
// independent of whether the middleware needed to actually use it is
// installed) and separately checks the local PATH for ssh-sk-helper (the
// OpenSSH FIDO2 middleware binary). Absent support is the expected common
// case and is reported as FIDOAbsent, never an error (T-01-03). The
// `ssh -Q key` call runs under a bounded exec.CommandContext timeout derived
// from ctx.
func probeFIDO(ctx context.Context) FIDOStatus {
	cctx, cancel := context.WithTimeout(ctx, probeTimeout)
	defer cancel()

	out, err := exec.CommandContext(cctx, "ssh", "-Q", "key").Output() // #nosec G204 -- fixed args, no user input
	if err != nil {
		return FIDOAbsent
	}

	skListed := false
	for _, tok := range parseKeyTypes(string(out)) {
		if strings.HasPrefix(tok, "sk-") {
			skListed = true
			break
		}
	}
	if !skListed {
		return FIDOAbsent
	}

	if _, err := exec.LookPath("ssh-sk-helper"); err != nil {
		return FIDOTokenListedOnly
	}
	return FIDOUsable
}
