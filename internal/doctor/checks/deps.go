package checks

import (
	"fmt"

	"github.com/castocolina/gitid/internal/doctor"
)

// CheckDeps checks whether the required and optional external tools are present
// on PATH. It returns one error finding per missing required tool (ssh,
// ssh-keygen, ssh-add, git) with a per-OS install hint (DOC-01), and one info
// finding when the optional clipboard tool is absent (D-05). Dep installs are
// report-only — no Fix descriptor is set (D-03).
//
// The function composes the injected deps.DetectTools seam so all behavior is
// fake-testable without touching the real PATH.
func CheckDeps(d doctor.Deps) []doctor.Finding {
	if d.DetectTools == nil {
		return nil
	}

	report := d.DetectTools()
	currentOS := ""
	if d.CurrentOS != nil {
		currentOS = d.CurrentOS()
	}

	var findings []doctor.Finding

	// Required tools: ssh, ssh-keygen, ssh-add, git. A missing required tool is
	// an error finding per D-05 (error = broken; authentication will fail).
	requiredTools := []struct {
		name    string
		present bool
	}{
		{"ssh", report.SSH},
		{"ssh-keygen", report.SSHKeygen},
		{"ssh-add", report.SSHAdd},
		{"git", report.Git},
	}

	for _, t := range requiredTools {
		if t.present {
			continue
		}
		hint := ""
		if d.InstallHint != nil {
			hint = d.InstallHint(t.name, currentOS)
		}
		findings = append(findings, doctor.Finding{
			Family:       doctor.FamilyDeps,
			Severity:     doctor.SeverityError,
			Title:        fmt.Sprintf("%s missing", t.name),
			Explanation:  "Required tool not found in PATH. gitid cannot function without it.",
			SuggestedFix: hint,
			Fix:          nil, // dep installs are report-only (D-03)
		})
	}

	// Optional tool: clipboard. A missing clipboard tool is an info finding per
	// D-05 (info = advisory; key copy-to-clipboard will not work but nothing breaks).
	if !report.Clipboard {
		clipHint := ""
		if d.InstallHint != nil {
			clipHint = d.InstallHint("clipboard", currentOS)
		}
		findings = append(findings, doctor.Finding{
			Family:       doctor.FamilyDeps,
			Severity:     doctor.SeverityInfo,
			Title:        "clipboard tool not found",
			Explanation:  "Optional tool not installed. Public-key copy to clipboard will not work.",
			SuggestedFix: clipHint,
			Fix:          nil, // dep installs are report-only (D-03)
		})
	}

	return findings
}
