// Package deps checks the availability of external tools required by gitid.
// Required tools: ssh, ssh-keygen, git. Optional tools: ssh-add, pbcopy,
// xclip, xsel, wl-copy. It returns a structured availability report used
// by the doctor and platform packages.
//
// Implementation lands in a later phase (Phase 2+).
package deps
