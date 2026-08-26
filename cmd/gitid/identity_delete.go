package main

// identity_delete.go implements `gitid identity delete <name>` — the CLI
// half of the Task 1 tracer. --git-only routes through the SAME runDelete
// the TUI's CommitDelete calls (D-02's one-chokepoint claim); --all surfaces
// identity.ErrScopeNotAvailable from that SAME call, never a second,
// CLI-local "not yet available" string (review R-16).

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"golang.org/x/term"

	"github.com/castocolina/gitid/internal/identity"
)

// identityDeleteFlags carries the four delete flags as a plain struct so
// runIdentityDelete stays independently testable without a *cobra.Command.
type identityDeleteFlags struct {
	GitOnly bool
	All     bool
	Yes     bool
	DryRun  bool
}

// newIdentityDeleteVerb builds the `delete <name>` verb spec.
func newIdentityDeleteVerb() identityVerb {
	var flags identityDeleteFlags
	return identityVerb{
		use:     "delete <name>",
		aliases: nil,
		short:   "Delete a gitid-managed identity",
		args:    cobra.ExactArgs(1),
		bindFlags: func(fs *pflag.FlagSet) {
			fs.BoolVar(&flags.GitOnly, "git-only", false, "delete only the Git side (SSH Host block, key pair, and allowed_signers line are left untouched)")
			fs.BoolVar(&flags.All, "all", false, "delete everything for the identity (SSH + Git + key) — not yet available in this build")
			fs.BoolVar(&flags.Yes, "yes", false, "skip the confirmation prompt; the timestamped backup is still taken unconditionally")
			fs.BoolVar(&flags.DryRun, "dry-run", false, "print the target list and exit 0 without writing")
		},
		run: func(cmd *cobra.Command, args []string) error {
			return runIdentityDelete(cmd, args[0], flags)
		},
	}
}

// runIdentityDelete implements the `--git-only` path against the SAME
// runDelete function the TUI's CommitDelete calls; the `--all` path surfaces
// whatever error runDelete(..., identity.DeleteScopeEverything) returns —
// currently identity.ErrScopeNotAvailable, wrapped, satisfying
// errors.Is(err, identity.ErrScopeNotAvailable) — from that one call, never
// duplicated here.
func runIdentityDelete(cmd *cobra.Command, name string, flags identityDeleteFlags) error {
	isTTY := term.IsTerminal(int(os.Stdout.Fd()))

	if flags.GitOnly && flags.All {
		return fmt.Errorf("gitid: --git-only and --all are mutually exclusive")
	}
	if !flags.GitOnly && !flags.All {
		if !isTTY {
			return fmt.Errorf("gitid: one of --git-only or --all is required in non-interactive mode")
		}
		return fmt.Errorf("gitid: one of --git-only or --all is required")
	}

	scope := identity.DeleteScopeGitOnly
	if flags.All {
		scope = identity.DeleteScopeEverything
	}

	home, err := resolveHomeForCLI()
	if err != nil {
		return err
	}
	b := newBackendForHome(home)

	if flags.DryRun {
		return printDeleteDryRun(cmd.OutOrStdout(), b, name, scope)
	}

	// Map the CLI's flags onto the lifecycle's three-valued confirmation
	// enum (review R2-03): --yes IS confirmationBypassedWithYes; an
	// interactive run installs the existing confirmDelete prompt under
	// confirmationRequired; a non-interactive run without --yes is refused
	// BEFORE the lifecycle (it can never be mistaken for pre-confirmed). The
	// CLI does NOT set confirmationAlreadyObtained — that value is reserved
	// for a layer that actually displayed the TUI's confirm screen.
	policy := lifecyclePolicy{Confirm: confirmationBypassedWithYes}
	if !flags.Yes {
		if !isTTY {
			return fmt.Errorf("gitid: refusing to delete %q without --yes in non-interactive mode", name)
		}
		policy = lifecyclePolicy{
			Confirm: confirmationRequired,
			Prompt: func(string) (bool, error) {
				return confirmDelete(cmd, name, scope)
			},
		}
	}

	// --yes suppresses ONLY the confirmation prompt above — the timestamped
	// backup itself is taken unconditionally by identity.Delete regardless
	// of --yes (D-02).
	res, derr := b.runDelete(name, scope, policy)
	if derr != nil {
		if len(res.Restored) > 0 {
			fmt.Fprintf(cmd.ErrOrStderr(), "restored: %s\n", strings.Join(res.Restored, "; ")) //nolint:errcheck // best-effort diagnostic
		}
		return derr
	}
	for _, bak := range res.Backups {
		fmt.Fprintf(cmd.OutOrStdout(), "backed up -> %s\n", b.displayPath(bak)) //nolint:errcheck // best-effort stdout
	}
	fmt.Fprintf(cmd.OutOrStdout(), "deleted %q (%s)\n", name, scope) //nolint:errcheck // best-effort stdout
	return nil
}

// confirmDelete prints a one-line prompt and reads a line from cmd's stdin,
// returning true only when the user typed exactly "yes".
func confirmDelete(cmd *cobra.Command, name string, scope identity.DeleteScope) (bool, error) {
	fmt.Fprintf(cmd.OutOrStdout(), "Delete identity %q (%s)? Type \"yes\" to confirm: ", name, scope) //nolint:errcheck // best-effort prompt
	reader := bufio.NewReader(cmd.InOrStdin())
	line, _ := reader.ReadString('\n')
	return strings.TrimSpace(line) == "yes", nil
}

// printDeleteDryRun prints the target list for name/scope and exits without
// writing (--dry-run). An unknown identity is still an error — a dry run
// must not silently succeed on a name that resolves to nothing.
func printDeleteDryRun(w io.Writer, b *realBackend, name string, scope identity.DeleteScope) error {
	acct, found := b.findAccount(name)
	if !found {
		return fmt.Errorf("gitid: no such identity: %q", name)
	}
	targets := []string{b.displayPath(b.gitconfigPath)}
	if acct.FragmentPath != "" {
		targets = append(targets, b.displayPath(acct.FragmentPath))
	}
	if scope == identity.DeleteScopeEverything {
		targets = append([]string{b.displayPath(b.storageTargetPath())}, targets...)
		if acct.KeyPath != "" {
			targets = append(targets, b.displayPath(acct.KeyPath), b.displayPath(acct.PubPath))
		}
		targets = append(targets, b.displayPath(b.allowedSigners))
	}
	fmt.Fprintf(w, "would delete %q (%s) — targets: %s\n", name, scope, strings.Join(targets, ", ")) //nolint:errcheck // best-effort stdout
	return nil
}
