package platform

import (
	"context"
	"fmt"
	"os/exec"
	"regexp"
	"strings"
	"time"
)

// probeTimeout bounds every external probe in this package (ssh -V,
// ssh -Q key, ssh-add -l, and the libfido2/ssh-sk-helper PATH lookup) so a
// hung agent or binary can never block gitid indefinitely (T-01-03). It is a
// var, not a const, so tests can shrink it to exercise real timeout behavior
// without waiting out the production default.
var probeTimeout = 3 * time.Second

// SSHVersion is the structured result of parsing `ssh -V` output. It is
// returned as a struct (never a pre-formatted string) so downstream callers
// (the KEY-01 catalog, the KEY-03 troubleshooting surface, and the D-08
// debug command) can render/compare individual fields without re-parsing a
// string.
type SSHVersion struct {
	OpenSSHVersion string
	SSLFlavor      string
	SSLVersion     string
	Raw            string
}

// sshVersionPattern extracts the OpenSSH version and SSL flavor/version from
// `ssh -V` output, e.g. "OpenSSH_9.7p1, LibreSSL 3.3.6" (macOS) or
// "OpenSSH_9.6p1, OpenSSL 3.0.13" (Linux). [VERIFIED: `ssh -V` run directly on
// the research/dev machine this session, OpenSSH_9.7p1].
var sshVersionPattern = regexp.MustCompile(`^OpenSSH_([^\s,]+),\s*(\S+)\s+(\S+)`)

// ProbeSSHVersion runs `ssh -V` and returns the parsed local OpenSSH
// version, SSL flavor (LibreSSL on macOS, OpenSSL on Linux), and SSL
// version as a structured SSHVersion. `ssh -V` writes to stderr and some
// builds exit non-zero even on success, so CombinedOutput is used and the
// error is only fatal when nothing was captured to parse. The probe runs
// under a bounded exec.CommandContext timeout (T-01-03: a hung `ssh`
// binary must never block gitid).
func ProbeSSHVersion() (SSHVersion, error) {
	ctx, cancel := context.WithTimeout(context.Background(), probeTimeout)
	defer cancel()
	out, err := exec.CommandContext(ctx, "ssh", "-V").CombinedOutput() // #nosec G204 -- fixed args, no user input
	raw := string(out)
	if err != nil && strings.TrimSpace(raw) == "" {
		return SSHVersion{}, fmt.Errorf("platform: probing ssh version via `ssh -V`: %w", err)
	}
	return parseSSHVersion(raw), nil
}

// parseSSHVersion is the pure, testable core of ProbeSSHVersion. Malformed
// or empty input returns a zero-value SSHVersion with Raw preserved, never a
// panic.
func parseSSHVersion(out string) SSHVersion {
	trimmed := strings.TrimSpace(out)
	v := SSHVersion{Raw: trimmed}
	m := sshVersionPattern.FindStringSubmatch(trimmed)
	if m == nil {
		return v
	}
	v.OpenSSHVersion = m[1]
	v.SSLFlavor = m[2]
	v.SSLVersion = m[3]
	return v
}
