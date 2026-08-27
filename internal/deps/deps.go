package deps

import (
	"fmt"
	"os/exec"
	"strings"
)

// Report is the structured availability of external tools gitid relies on.
// Required tools (SSH, SSHKeygen, Git) must be present; optional tools
// (SSHAdd, Clipboard) enhance behavior but never block a required operation.
type Report struct {
	SSH       bool
	SSHKeygen bool
	Git       bool
	SSHAdd    bool
	Clipboard bool
}

// MissingRequired returns the names of required tools that were not found,
// in the fixed order ssh, ssh-keygen, git. Optional tools never appear here.
func (r Report) MissingRequired() []string {
	var missing []string
	if !r.SSH {
		missing = append(missing, "ssh")
	}
	if !r.SSHKeygen {
		missing = append(missing, "ssh-keygen")
	}
	if !r.Git {
		missing = append(missing, "git")
	}
	return missing
}

// found reports whether a tool resolves on the current PATH.
func found(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}

// GitVersion returns the installed git version token — the first field after
// "git version" in `git --version` output — or an error when git is not on
// PATH or its output is unparseable. It is the ONE git-version probe in the
// module: GitVersionAtLeast below reads through it, and
// globalgit.VersionGate receives its result (D-08 mandates reusing this probe
// rather than opening a second one). Unlike GitVersionAtLeast, it fails loudly
// on an unreadable version, which is exactly the signal a HARD gate needs.
func GitVersion() (string, error) {
	cmd := exec.Command("git", "--version") //nolint:gosec // arg-slice form, no shell; fixed argument (G204)
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("running git --version: %w", err)
	}
	line := strings.TrimSpace(string(out))
	parts := strings.Fields(line)
	if len(parts) < 3 {
		return "", fmt.Errorf("unexpected git --version output %q", line)
	}
	return parts[2], nil
}

// GitVersionAtLeast reports whether the installed git binary is at least the
// given major.minor version. It is used for feature gates such as the
// merge.conflictstyle=zdiff3 gate (requires git >= 2.35, RESEARCH C4). On any
// error (git not found, unexpected output) it returns true so callers default to
// including the feature rather than silently omitting it. That optimistic
// fallback is correct for a feature that is merely ignored when unsupported —
// NOT for the hard gate, whose callers must consult GitVersion itself (the
// conservative direction) instead of this boolean.
func GitVersionAtLeast(major, minor int) bool {
	v, err := GitVersion()
	if err != nil {
		return true // optimistic fallback: assume modern git
	}
	gMajor, gMinor := gitVersionParts(v)
	if gMajor != major {
		return gMajor > major
	}
	return gMinor >= minor
}

// gitVersionParts parses a "major.minor" token into its numeric components.
func gitVersionParts(v string) (major, minor int) {
	parts := strings.SplitN(v, ".", 3)
	if len(parts) > 0 {
		_, _ = fmt.Sscanf(parts[0], "%d", &major)
	}
	if len(parts) > 1 {
		_, _ = fmt.Sscanf(parts[1], "%d", &minor)
	}
	return major, minor
}

// Detect probes the local PATH for each required and optional tool and
// returns a populated Report. Clipboard is true when any platform clipboard
// helper (pbcopy/wl-copy/xclip/xsel) is available.
func Detect() Report {
	return Report{
		SSH:       found("ssh"),
		SSHKeygen: found("ssh-keygen"),
		Git:       found("git"),
		SSHAdd:    found("ssh-add"),
		Clipboard: found("pbcopy") || found("wl-copy") || found("xclip") || found("xsel"),
	}
}
