package main

// health.go is the minimal `gitid health [--json]` command (08-01-PLAN.md
// Task 1/2): it calls the SAME buildDoctorDeps(home) + doctor.Run(deps) +
// doctorFindings(home) construction the TUI's Health/Fixer tabs consume via
// realBackend.InitialState() — one source, three consumers. The --json
// shape here is a PROVISIONAL flat array for this tracer/split wave only;
// Wave 7 (08-07 Task 2) supersedes it with this project's versioned-envelope
// convention (matching the `gitid ssh`/`gitid git` JSON commands). Do not
// treat this array shape as a stable contract.

import (
	"fmt"
	"io"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/castocolina/gitid/internal/tuikit"
)

// newHealthCmd builds `gitid health [--json] [--identity NAME]`. --identity
// scopes the findings to one identity via Finding.IdentityName (D-04,
// HLTH-05), with real identity-name shell completion (mirroring the
// existing identity-noun completion pattern). Per D-04's Claude's-discretion
// note, global (empty-IdentityName) findings are EXCLUDED entirely from the
// scoped view — the same choice internal/tuikit/identities.go's TUI deep-link
// makes (health_screen.go's healthModel.findings scopes via
// tuikit.FindingsFor, which only matches DemoFinding.Identity == name and
// therefore never includes a global finding either); documented here so the
// TUI and the CLI stay consistent per D-04's requirement.
func newHealthCmd() *cobra.Command {
	var jsonOut bool
	var identityName string
	cmd := newVerbCmd(identityVerb{
		use:   "health",
		short: "Show identity/config health findings",
		args:  cobra.NoArgs,
		bindFlags: func(fs *pflag.FlagSet) {
			fs.BoolVar(&jsonOut, "json", false, "print findings as a JSON array (provisional shape — superseded in a later phase 8 wave)")
			fs.StringVar(&identityName, "identity", "", "scope findings to one identity's own SSH + Git findings (D-04); global findings are excluded from the scoped view")
		},
		run: func(cmd *cobra.Command, _ []string) error {
			home, err := resolveHomeForCLI()
			if err != nil {
				return err
			}
			findings := suppressParseErrorFindings(doctorFindings(home))
			if identityName != "" {
				findings = findingsForIdentity(findings, identityName)
			}
			if jsonOut {
				return writeJSON(cmd.OutOrStdout(), findings)
			}
			return printHealthFindings(cmd.OutOrStdout(), findings)
		},
	})
	//nolint:errcheck // completion registration failure is non-fatal (cobra ignores it gracefully)
	_ = cmd.RegisterFlagCompletionFunc("identity", func(_ *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
		home, err := resolveHomeForCLI()
		if err != nil {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		b := newBackendForHome(home)
		var names []string
		for _, a := range b.accounts() {
			names = append(names, a.Name)
		}
		return names, cobra.ShellCompDirectiveNoFileComp
	})
	return cmd
}

// findingsForIdentity filters findings to exactly identityName's own
// findings (Finding.IdentityName) — global (empty-Identity) findings are
// excluded, the same choice the TUI deep-link's healthModel.findings makes
// via tuikit.FindingsFor.
func findingsForIdentity(findings []tuikit.DemoFinding, identityName string) []tuikit.DemoFinding {
	out := make([]tuikit.DemoFinding, 0, len(findings))
	for _, f := range findings {
		if f.Identity == identityName {
			out = append(out, f)
		}
	}
	return out
}

func suppressParseErrorFindings(findings []tuikit.DemoFinding) []tuikit.DemoFinding {
	blocked := make(map[string]bool)
	for _, finding := range findings {
		if finding.Family == "Files" && finding.Severity == tuikit.SeverityCritical {
			blocked[finding.Section] = true
		}
	}
	filtered := make([]tuikit.DemoFinding, 0, len(findings))
	for _, finding := range findings {
		if blocked[finding.Section] && finding.Family != "Files" {
			continue
		}
		filtered = append(filtered, finding)
	}
	return filtered
}

// printHealthFindings renders one line per finding: severity, family,
// section (SSH/Git), and title.
func printHealthFindings(w io.Writer, findings []tuikit.DemoFinding) error {
	if len(findings) == 0 {
		_, err := fmt.Fprintln(w, "no findings")
		return err
	}
	for _, f := range findings {
		if _, err := fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", f.Severity, f.Section, f.Family, f.Title); err != nil {
			return err
		}
	}
	return nil
}
