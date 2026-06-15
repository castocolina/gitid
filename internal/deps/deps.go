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

// GitVersionAtLeast reports whether the installed git binary is at least the
// given major.minor version. It is used for feature gates such as the
// merge.conflictstyle=zdiff3 gate (requires git >= 2.35, RESEARCH C4). On any
// error (git not found, unexpected output) it returns true so callers default to
// including the feature rather than silently omitting it.
func GitVersionAtLeast(major, minor int) bool {
	cmd := exec.Command("git", "--version") //nolint:gosec // arg-slice form, no shell; fixed argument (G204)
	out, err := cmd.Output()
	if err != nil {
		return true // optimistic fallback: assume modern git
	}
	line := strings.TrimSpace(string(out))
	parts := strings.Fields(line)
	if len(parts) < 3 {
		return true
	}
	vparts := strings.SplitN(parts[2], ".", 3)
	if len(vparts) < 2 {
		return true
	}
	var maj, minV int
	if _, err := fmt.Sscanf(vparts[0], "%d", &maj); err != nil {
		return true
	}
	if _, err := fmt.Sscanf(vparts[1], "%d", &minV); err != nil {
		return true
	}
	if maj != major {
		return maj > major
	}
	return minV >= minor
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
