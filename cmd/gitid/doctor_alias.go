package main

import "github.com/spf13/cobra"

// newDoctorAliasCmd builds the hidden Docker-style compatibility alias. Plain
// `doctor` calls the same health body; `doctor --fix` selects fix mode only,
// while --yes and --dry-run pass unchanged to the same fix body. In particular,
// --fix alone remains interactive and never implies automatic application.
func newDoctorAliasCmd() *cobra.Command {
	var fix, yes, dryRun, jsonOut bool
	var identityName string
	cmd := &cobra.Command{
		Use:    "doctor",
		Short:  "Compatibility alias for health",
		Hidden: true,
		Args:   cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if fix {
				home, err := resolveHomeForCLI()
				if err != nil {
					return err
				}
				return runFixCommand(cmd.OutOrStdout(), cmd.InOrStdin(), home, yes, dryRun)
			}
			return runHealth(cmd.OutOrStdout(), identityName, jsonOut)
		},
	}
	cmd.Flags().BoolVar(&fix, "fix", false, "apply suggested health fixes")
	cmd.Flags().BoolVar(&yes, "yes", false, "apply every fixable finding without a per-finding prompt")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "preview every fixable finding's diff and exit without writing")
	cmd.Flags().BoolVar(&jsonOut, "json", false, "print the gitid.health/v1 document")
	cmd.Flags().StringVar(&identityName, "identity", "", "scope findings to one identity")
	return cmd
}
