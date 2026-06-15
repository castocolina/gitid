// Command gitid manages Git identities by coordinating SSH and Git configuration.
package main

import (
	"os"

	"github.com/spf13/cobra"
)

const version = "0.0.0-dev"

func main() {
	if err := Execute(); err != nil {
		// IN-03: propagate the tiered doctor exit code (0/1/2/3) instead of
		// collapsing to a flat 1. doctorExitCode is set by the doctor RunE
		// before it returns a non-nil error; all other commands leave it 0,
		// so we fall back to 1 for non-doctor errors (no regression).
		code := doctorExitCode
		if code == 0 {
			code = 1
		}
		os.Exit(code)
	}
}

// Execute builds the root command and runs it, returning any error so main()
// owns the single exit point (thin main()->Execute() indirection preserved).
func Execute() error {
	return newRootCmd().Execute()
}

// newRootCmd assembles the gitid Cobra command tree: the root, the `identity`
// group, and its `add` / `test` subcommands. Cobra auto-registers a
// `completion` subcommand for bash/zsh/fish/PowerShell.
func newRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:           "gitid",
		Short:         "Manage multiple Git identities by coordinating SSH and Git configuration",
		Version:       version,
		SilenceUsage:  true,
		SilenceErrors: false,
	}

	identity := &cobra.Command{
		Use:   "identity",
		Short: "Create and verify Git identities",
	}
	identity.AddCommand(newAddCmd())
	identity.AddCommand(newListCmd())
	identity.AddCommand(newTestCmd())
	identity.AddCommand(newRotateCmd())
	identity.AddCommand(newUpdateCmd())
	identity.AddCommand(newDeleteCmd())
	root.AddCommand(identity)

	baseline := &cobra.Command{
		Use:   "baseline",
		Short: "Manage the shared global git baseline (core/push/pull defaults, gitignore, url rewrites)",
	}
	baseline.AddCommand(newBaselineSetupCmd())
	baseline.AddCommand(newBaselineShowCmd())
	root.AddCommand(baseline)

	root.AddCommand(newDoctorCmd())

	return root
}
