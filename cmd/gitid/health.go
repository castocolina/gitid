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

func newHealthCmd() *cobra.Command {
	var jsonOut bool
	return newVerbCmd(identityVerb{
		use:   "health",
		short: "Show identity/config health findings",
		args:  cobra.NoArgs,
		bindFlags: func(fs *pflag.FlagSet) {
			fs.BoolVar(&jsonOut, "json", false, "print findings as a JSON array (provisional shape — superseded in a later phase 8 wave)")
		},
		run: func(cmd *cobra.Command, _ []string) error {
			home, err := resolveHomeForCLI()
			if err != nil {
				return err
			}
			findings := doctorFindings(home)
			if jsonOut {
				return writeJSON(cmd.OutOrStdout(), findings)
			}
			return printHealthFindings(cmd.OutOrStdout(), findings)
		},
	})
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
