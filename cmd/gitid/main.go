// Command gitid manages Git identities by coordinating SSH and Git configuration.
package main

import (
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/castocolina/gitid/tui"
)

const version = "0.0.0-dev"

// noArgsAction handles the no-args case for main(): if isTTY is true, calls
// run() (e.g. tui.Run) and returns 0 on success or 1 on error; if isTTY is
// false, writes the usage hint to errw and returns 1 (TUI-01 non-TTY contract,
// UI-SPEC §"Non-TTY / Piped Behavior Contract").
//
// Extracted as a named helper so tests can drive both branches without
// invoking the real TUI or os.Exit.
func noArgsAction(isTTY bool, run func() error, out io.Writer, errw io.Writer) int {
	if !isTTY {
		_, _ = fmt.Fprintln(errw, "gitid: no subcommand given. Run 'gitid --help' for usage.")
		return 1
	}
	if err := run(); err != nil {
		_, _ = fmt.Fprintf(errw, "gitid: tui: %v\n", err)
		return 1
	}
	_ = out // out is reserved for future use; currently no TTY success output
	return 0
}

func main() {
	if len(os.Args) == 1 {
		// No subcommand: branch on TTY — launch TUI or print usage hint.
		isTTY := term.IsTerminal(int(os.Stdout.Fd()))
		code := noArgsAction(isTTY, tui.Run, os.Stdout, os.Stderr)
		os.Exit(code)
	}
	if err := Execute(); err != nil {
		// A real command error (e.g. "doctor: --yes requires --fix", or any other
		// command's failure). Cobra has already printed it. Exit non-zero,
		// preferring the tiered doctor code when one was set.
		code := doctorExitCode
		if code == 0 {
			code = 1
		}
		os.Exit(code)
	}
	// IN-03: propagate the tiered doctor exit code (0/1/2/3) on a clean Execute.
	// doctor RunE stores it in doctorExitCode and returns nil (so Cobra prints no
	// spurious "Error: exit code N"); all other commands leave it 0.
	if doctorExitCode != 0 {
		os.Exit(doctorExitCode)
	}
}

// Execute builds the root command and runs it, returning any error so main()
// owns the single exit point (thin main()->Execute() indirection preserved).
func Execute() error {
	return newRootCmd().Execute()
}

// newRootCmd assembles the gitid Cobra command tree: the root, the `identity`
// group, and its subcommands. Cobra auto-registers a `completion` subcommand
// for bash/zsh/fish/PowerShell (D-08/CLI-02).
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
	// D-06: identity copy subcommand (canonical; top-level copy is an alias below)
	identity.AddCommand(newIdentityCopyCmd())
	root.AddCommand(identity)

	baseline := &cobra.Command{
		Use:   "baseline",
		Short: "Manage the shared global git baseline (core/push/pull defaults, gitignore, url rewrites)",
	}
	baseline.AddCommand(newBaselineSetupCmd())
	baseline.AddCommand(newBaselineShowCmd())
	root.AddCommand(baseline)

	root.AddCommand(newDoctorCmd())

	// D-05: top-level rotate alias — delegates to same handler as identity rotate.
	rotateTL := &cobra.Command{
		Use:   "rotate <name>",
		Short: "Rotate the SSH key for an identity and re-test all artifacts",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runIdentityRotate(cmd.InOrStdin(), cmd.OutOrStdout(), args[0], false, buildDeps)
		},
	}
	root.AddCommand(rotateTL)

	// D-06: top-level copy alias
	root.AddCommand(newCopyCmd())

	// D-07: host group with add subcommand
	host := &cobra.Command{
		Use:   "host",
		Short: "Manage SSH host aliases",
	}
	host.AddCommand(newHostAddCmd())
	root.AddCommand(host)

	return root
}
