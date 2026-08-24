package tester

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

// Outcome is the three-way classification of an SSH connectivity test, derived
// strictly from output substrings (never the exit code — Pitfall 2 / D-01).
type Outcome int

const (
	// PASS — the key is already authorized: "successfully authenticated".
	PASS Outcome = iota
	// ReachableNotUploaded — host reachable but key not yet uploaded:
	// "Permission denied (publickey)". Expected for a brand-new key; the create
	// flow proceeds (D-02).
	ReachableNotUploaded
	// Failure — connection refused, DNS failure, timeout, etc. Abort, no write.
	Failure
)

// Result is the structured output of a connectivity test. It always carries the
// exact command run (input) and the raw combined output (TEST-03).
type Result struct {
	Command          string
	Output           string
	ResolutionOutput string
	Outcome          Outcome
}

// ResolvedConfig holds the lowercase-keyed values parsed from `ssh -G` (D-03).
type ResolvedConfig struct {
	User           string
	Hostname       string
	Port           string
	IdentitiesOnly string
	IdentityFiles  []string
}

// runner is the injectable command-execution seam used by preWriteWith so unit
// tests can drive the classifier without live network access. It receives the
// fully-formed ssh argument slice and returns combined output; the SSH exit code
// is intentionally discarded by callers (Pitfall 2 / D-01).
type runner func(args []string) (string, error)

// ClassifyPreWrite maps combined SSH output to an Outcome using substring
// matching only. The exit code is never consulted: `ssh -T` exits 0 even when it
// prints "Permission denied (publickey)" (verified — Pitfall 2 / D-01).
func ClassifyPreWrite(combinedOutput string) Outcome {
	switch {
	case strings.Contains(combinedOutput, "successfully authenticated"):
		return PASS
	case strings.Contains(combinedOutput, "Permission denied (publickey)"):
		return ReachableNotUploaded
	default:
		return Failure
	}
}

// PreWriteCommand returns the string representation of the ssh command that
// PreWrite would run for the given keyPath, hostname, and port. It is a pure
// read-only helper (no exec) that calls exec.Command("ssh", preWriteArgs(...)).String()
// so the returned string is byte-identical to Result.Command for the same inputs.
// Callers use this to display the exact pre-run command before PreWrite executes.
func PreWriteCommand(keyPath, hostname string, port int, knownHostsPath string) string {
	args := preWriteArgs(keyPath, hostname, port, knownHostsPath)
	cmd := exec.Command("ssh", args...) //nolint:gosec // arg-slice form for cmd.String() display; not executed here
	return cmd.String()
}

// preWriteArgs builds the explicit-key pre-write ssh argument slice. Arguments
// are passed as a slice (never a shell string), keeping the call gosec
// G204-clean and free of OS-command-injection risk (threat T-02-18).
func preWriteArgs(keyPath, hostname string, port int, knownHostsPath string) []string {
	args := []string{
		"-i", keyPath,
		"-o", "IdentitiesOnly=yes",
		"-o", "BatchMode=yes",
		"-o", "ConnectTimeout=10",
		"-o", "StrictHostKeyChecking=accept-new",
		"-p", strconv.Itoa(port),
		"-T", "git@" + hostname,
	}
	if knownHostsPath != "" {
		args = append(args, "-o", "UserKnownHostsFile="+knownHostsPath)
	}
	return args
}

// resolvedViaArgs builds the stage-2 CONNECTIVITY ssh argument slice used by
// ResolvedVia: the alias is resolved through the staged temp config (-F) and
// the config's IdentityFile supplies the key — NO explicit -i. This proves the
// alias resolves to the right key through the managed block alone (TEST-02).
// It is the single source of truth shared by the executing path (ResolvedVia)
// and the display path (ResolvedViaCommand), so the command shown to the user
// can never drift from the command run (TEST-01). The `ssh -G` resolution call
// is deliberately NOT built here.
//
// Arguments are passed as a slice (never a shell string), keeping the call
// gosec G204-clean and free of OS-command-injection risk (threat T-03-03).
func resolvedViaArgs(configPath, keyPath, alias string, knownHostsPath string) []string {
	_ = keyPath // keyPath is not pinned on the command line; the staged config supplies it.
	args := []string{
		"-F", configPath,
		"-o", "IdentitiesOnly=yes",
		"-o", "BatchMode=yes",
		"-o", "ConnectTimeout=10",
		"-T", "git@" + alias,
	}
	if knownHostsPath != "" {
		args = append(args, "-o", "UserKnownHostsFile="+knownHostsPath)
	}
	return args
}

// ResolvedViaCommand returns the string representation of the stage-2
// connectivity command ResolvedVia would run for the given staged config, key
// and alias. It is a pure read-only helper (no exec) built from the SAME
// resolvedViaArgs slice ResolvedVia executes, so the returned string is
// byte-identical to that call's Result.Command for the same inputs — the
// stage-2 counterpart of PreWriteCommand, closing the "shown command == run
// command" contract (TEST-01) for both test stages.
func ResolvedViaCommand(configPath, keyPath, alias string, knownHostsPath string) string {
	args := resolvedViaArgs(configPath, keyPath, alias, knownHostsPath)
	cmd := exec.Command("ssh", args...) //nolint:gosec // arg-slice form for cmd.String() display; not executed here
	return cmd.String()
}

// preWriteWith runs the pre-write test through an injected runner and assembles
// the Result, capturing the exact command string (input) and raw output (TEST-03)
// and classifying strictly by output substring (exit code ignored).
func preWriteWith(run runner, keyPath, hostname string, port int, knownHostsPath string) Result {
	args := preWriteArgs(keyPath, hostname, port, knownHostsPath)
	out, _ := run(args)                 // exit code intentionally ignored (Pitfall 2 / D-01)
	cmd := exec.Command("ssh", args...) //nolint:gosec // arg-slice form for cmd.String() display; not executed here
	return Result{
		Command: cmd.String(),
		Output:  out,
		Outcome: ClassifyPreWrite(out),
	}
}

// PreWrite runs the explicit-key pre-write connectivity test:
//
//	ssh -i <key> -o IdentitiesOnly=yes -o BatchMode=yes -o ConnectTimeout=10 \
//	    -o StrictHostKeyChecking=accept-new -o UserKnownHostsFile=<staging> \
//	    -p <port> -T git@<hostname>
//
// It captures combined stdout+stderr, ignores the (unreliable) exit code, and
// returns a Result with the input command, raw output, and substring-derived
// outcome. Read-only: it never mutates any file.
func PreWrite(keyPath, hostname string, port int, knownHostsPath string) Result {
	return preWriteWith(execRunner, keyPath, hostname, port, knownHostsPath)
}

// execRunner is the production runner: it runs `ssh <args...>` with arguments
// passed as a slice (no shell) and returns the combined output. The SSH exit
// code is deliberately swallowed here — callers classify by output (D-01).
func execRunner(args []string) (string, error) {
	out, err := exec.Command("ssh", args...).CombinedOutput() //nolint:gosec // arg-slice form, no shell; host/key derived from validated gitid input (G204)
	return string(out), err
}

// Resolved runs the resolved-config phase for an alias: a live `ssh -T git@<alias>`
// connectivity test plus an `ssh -G <alias>` parse. It returns the connectivity
// Result and the parsed ResolvedConfig. Read-only.
func Resolved(alias string) (Result, ResolvedConfig) {
	args := []string{"-o", "BatchMode=yes", "-o", "ConnectTimeout=10", "-T", "git@" + alias}
	out, _ := execRunner(args)          // exit code ignored (D-01)
	cmd := exec.Command("ssh", args...) //nolint:gosec // arg-slice form for cmd.String() display; not executed here
	res := Result{Command: cmd.String(), Output: out, Outcome: ClassifyPreWrite(out)}

	gOut, _ := exec.Command("ssh", "-G", alias).Output() //nolint:gosec // arg-slice form, no shell; alias is validated gitid input (G204)
	res.ResolutionOutput = string(gOut)
	return res, ParseResolved(res.ResolutionOutput)
}

// ResolvedVia runs the resolved-config phase against an EXPLICIT ssh config file
// and key, without touching the user's ~/.ssh/config:
//
//	ssh -F <configPath> -i <keyPath> -o IdentitiesOnly=yes -o BatchMode=yes \
//	    -o ConnectTimeout=10 -o UserKnownHostsFile=<staging> -T git@<alias>
//
// The staged temp config carries the identity's Host block (alt-SSH hostname/port,
// IdentityFile = the staged key), so the alias resolves through that block — the
// same way it will once the managed block is written to the real config, but
// proven first in isolation. This is how the create wizard tests a typed alias
// BEFORE the block ever lands in the live file (UAT G-5): the alias is not a DNS
// name, so it can only resolve once a Host stanza exists somewhere ssh reads.
//
// Read-only with respect to ~/.ssh/config and the user's known_hosts: every call
// is isolated to the provided knownHostsPath. The `ssh -G` parse also uses -F so
// the returned ResolvedConfig reflects the staged block, not the live file.
func ResolvedVia(configPath, keyPath, alias string, knownHostsPath string) (Result, ResolvedConfig) {
	// Shared with ResolvedViaCommand so the displayed stage-2 command is exactly
	// what runs here (TEST-01).
	args := resolvedViaArgs(configPath, keyPath, alias, knownHostsPath)
	out, _ := execRunner(args)          // exit code ignored (D-01)
	cmd := exec.Command("ssh", args...) //nolint:gosec // arg-slice form for cmd.String() display; not executed here
	res := Result{Command: cmd.String(), Output: out, Outcome: ClassifyPreWrite(out)}

	gOut, _ := exec.Command("ssh", "-F", configPath, "-G", alias).Output() //nolint:gosec // arg-slice form, no shell; paths/alias are validated gitid input (G204)
	res.ResolutionOutput = string(gOut)
	return res, ParseResolved(res.ResolutionOutput)
}

// ResolvedViaGCommand returns the string representation of the stage-2
// ssh -G resolution command that ResolvedVia runs to prove the alias resolves
// through the staged config to the expected key. It is a pure read-only helper
// (no exec) built from the same config-path and alias arguments as ResolvedVia,
// so the command shown to the user is byte-identical to what was run (TEST-01).
//
// This is deliberately separate from ResolvedViaCommand (the connectivity call)
// so callers can render BOTH commands and BOTH raw outputs to the user (CR-07).
func ResolvedViaGCommand(configPath, alias string) string {
	args := []string{"-F", configPath, "-G", alias}
	cmd := exec.Command("ssh", args...) //nolint:gosec // arg-slice form for cmd.String() display; not executed here
	return cmd.String()
}

// ExpectedResolution holds the field values the stage-2 ssh -G proof must
// match for the result to be accepted as a valid resolution (CR-07/TEST-02).
// All fields are compared after lower-casing the parsed output values.
type ExpectedResolution struct {
	// User is the expected SSH User directive — always "git" for managed
	// identities.
	User string
	// Hostname is the real endpoint hostname the alias resolves to.
	Hostname string
	// Port is the expected port as a decimal string.
	Port string
	// IdentitiesOnly is "yes" for every gitid-managed host block.
	IdentitiesOnly string
	// ExpectedKeyPath is the path that must appear as the FIRST IdentityFile
	// in the resolved config's effective key list. A key listed later (after a
	// default ~/.ssh/id_ed25519) means the config wiring is wrong.
	ExpectedKeyPath string
}

// ValidateResolvedConfig validates the parsed ssh -G output against the
// expected resolution parameters. It returns a descriptive error naming the
// mismatched field when any field fails, and nil when all fields match (CR-07).
//
// Failure cases:
//   - Any expected field is non-empty but the corresponding resolved value is
//     empty (the ssh -G output was absent or truncated).
//   - The resolved User, Hostname, Port, or IdentitiesOnly does not match.
//   - ExpectedKeyPath is non-empty and does not appear as the FIRST element of
//     IdentityFiles (the expected key is not the effective key).
func ValidateResolvedConfig(rc ResolvedConfig, expected ExpectedResolution) error {
	if expected.User != "" && rc.User != expected.User {
		return fmt.Errorf("stage-2 proof: user mismatch: got %q, want %q", rc.User, expected.User)
	}
	if expected.Hostname != "" && rc.Hostname != expected.Hostname {
		return fmt.Errorf("stage-2 proof: hostname mismatch: got %q, want %q", rc.Hostname, expected.Hostname)
	}
	if expected.Port != "" && rc.Port != expected.Port {
		return fmt.Errorf("stage-2 proof: port mismatch: got %q, want %q", rc.Port, expected.Port)
	}
	if expected.IdentitiesOnly != "" && rc.IdentitiesOnly != expected.IdentitiesOnly {
		return fmt.Errorf("stage-2 proof: identitiesonly mismatch: got %q, want %q", rc.IdentitiesOnly, expected.IdentitiesOnly)
	}
	if expected.ExpectedKeyPath != "" {
		if len(rc.IdentityFiles) == 0 {
			return fmt.Errorf("stage-2 proof: no identityfile in ssh -G output (empty or truncated output?)")
		}
		if rc.IdentityFiles[0] != expected.ExpectedKeyPath {
			return fmt.Errorf("stage-2 proof: first effective identityfile = %q, want %q (earlier key shadows the expected one)", rc.IdentityFiles[0], expected.ExpectedKeyPath)
		}
	}
	return nil
}

// ParseResolved parses `ssh -G` output. Keys are matched on a lowercase
// `<key> ` prefix, case-sensitive (Pitfall 3): camelCase lines never match.
// identityfile may appear multiple times and all occurrences are collected (D-03).
func ParseResolved(sshGOutput string) ResolvedConfig {
	var rc ResolvedConfig
	for _, line := range strings.Split(sshGOutput, "\n") {
		switch {
		case strings.HasPrefix(line, "identityfile "):
			rc.IdentityFiles = append(rc.IdentityFiles, strings.TrimSpace(line[len("identityfile "):]))
		case strings.HasPrefix(line, "identitiesonly "):
			rc.IdentitiesOnly = strings.TrimSpace(line[len("identitiesonly "):])
		case strings.HasPrefix(line, "user "):
			rc.User = strings.TrimSpace(line[len("user "):])
		case strings.HasPrefix(line, "hostname "):
			rc.Hostname = strings.TrimSpace(line[len("hostname "):])
		case strings.HasPrefix(line, "port "):
			rc.Port = strings.TrimSpace(line[len("port "):])
		}
	}
	return rc
}
