package main

import (
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"

	"github.com/castocolina/gitid/internal/tester"
)

// newTestCmd builds `gitid identity test <name>` — the reusable resolved-test
// entry point (D-04). It treats the positional argument as the host alias and
// runs the resolved two-phase test, printing each command (input) and its real
// output (TEST-02/TEST-03). It contains no business logic: it delegates to
// internal/tester.Resolved.
func newTestCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "test <name|alias>",
		Short: "Re-run the resolved ssh -T / ssh -G test for an identity alias",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runIdentityTest(cmd.OutOrStdout(), args[0])
		},
	}
}

// runIdentityTest runs tester.Resolved for alias and renders the connectivity
// command + output and the parsed `ssh -G` assertions to out (TEST-02/TEST-03).
func runIdentityTest(out io.Writer, alias string) error {
	if strings.TrimSpace(alias) == "" {
		return fmt.Errorf("identity test: alias must not be empty")
	}

	res, resolved := tester.Resolved(alias)

	fp(out, fmt.Sprintf("$ %s\n%s\n", res.Command, strings.TrimRight(res.Output, "\n")))
	fp(out, fmt.Sprintf("$ ssh -G %s\n", alias))
	fp(out, fmt.Sprintf("  user           %s\n", resolved.User))
	fp(out, fmt.Sprintf("  hostname       %s\n", resolved.Hostname))
	fp(out, fmt.Sprintf("  port           %s\n", resolved.Port))
	fp(out, fmt.Sprintf("  identitiesonly %s\n", resolved.IdentitiesOnly))
	for _, f := range resolved.IdentityFiles {
		fp(out, fmt.Sprintf("  identityfile   %s\n", f))
	}
	return nil
}
