package deps

import "os/exec"

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
