package main

// identity_delete.go implements `gitid identity delete <name>` — the CLI half
// of the delete lifecycle. --git-only routes through the SAME runDelete the
// TUI's CommitDelete calls (D-02's one-chokepoint claim); --all completes the
// everything scope plan 05-01 deliberately left refusing. The headless run
// prints the full DeletePlan — targets, shared-key note, scan hits,
// disclaimer — before acting, so it discloses what the TUI's confirm screen
// would have shown (threat T-05-38).

import (
	"bufio"
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/castocolina/gitid/internal/identity"
	"github.com/castocolina/gitid/internal/tuikit"
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
			fs.BoolVar(&flags.All, "all", false, "delete everything for the identity (SSH + Git + key)")
			fs.BoolVar(&flags.Yes, "yes", false, "skip the confirmation prompt; the timestamped backup is still taken unconditionally")
			fs.BoolVar(&flags.DryRun, "dry-run", false, "print the full delete plan and exit 0 without writing anything")
		},
		run: func(cmd *cobra.Command, args []string) error {
			return runIdentityDelete(cmd, args[0], flags, termIsStdinTTY(), termIsStdoutTTY())
		},
	}
}

// cliDeleteInto is the CLI's delete lifecycle seam: a thin adapter over the
// SAME runDelete the TUI's CommitDelete calls. test-only var so a recording
// double can prove the handler invokes the lifecycle exactly once and that a
// refused run invokes it ZERO times.
var cliDeleteInto = func(b *realBackend, name string, scope identity.DeleteScope, p lifecyclePolicy) (lifecycleResult, error) {
	return b.runDelete(name, scope, p)
}

// runIdentityDelete implements the CLI delete verb. Exactly one scope flag is
// required in EVERY mode (both or neither is an error — a delete may never
// silently pick a scope). A dry run prints the full plan and exits; a real
// run maps the flags onto the confirmation enum and calls the SAME runDelete
// the TUI's CommitDelete reaches, printing the plan first so even the
// headless --yes path discloses what it is about to remove.
func runIdentityDelete(cmd *cobra.Command, name string, flags identityDeleteFlags, stdinTTY, stdoutTTY bool) error {
	if flags.GitOnly && flags.All {
		return fmt.Errorf("gitid: cannot pass both --git-only and --all: delete requires exactly one scope flag")
	}
	if !flags.GitOnly && !flags.All {
		return fmt.Errorf("gitid: exactly one of --git-only or --all is required")
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
		// A dry run needs no authorization: it stops before the confirmation
		// gate and writes nothing, so it is accepted without a terminal and
		// without --yes (the per-verb dry-run contract row for delete).
		return printDeleteDryRun(cmd.OutOrStdout(), b, name, scope)
	}

	// The authorization gate runs BEFORE a policy is constructed or any
	// lifecycle function is called (review R2-03): a non-interactive
	// destructive invocation without --yes is refused here, and the
	// regression test proves no lifecycle function is invoked. A missing
	// confirmation is treated exactly like a missing required flag.
	if !flags.Yes && (!stdinTTY || !stdoutTTY) {
		return fmt.Errorf("gitid: refusing to delete %q without --yes in non-interactive mode (use --yes to skip only the confirmation prompt; the timestamped backup is still taken)", name)
	}

	// Print the delete plan before acting so the headless run discloses what
	// the confirm screen would have shown (threat T-05-38) — targets, the
	// shared-key downgrade note, the scan hits, and the disclaimer.
	if plan, perr := b.DeletePlan(name, string(scope)); perr == nil {
		if rerr := renderDeletePlan(cmd.OutOrStdout(), b, plan, "will delete"); rerr != nil {
			return rerr
		}
	}

	policy, perr := confirmationPolicyFrom(cmd, "delete "+name, stdinTTY, stdoutTTY, flags.Yes, func() (bool, error) {
		return confirmDelete(cmd, name, scope)
	})
	if perr != nil {
		return perr
	}

	// --yes suppresses ONLY the confirmation prompt above — the timestamped
	// backup itself is taken unconditionally by identity.Delete regardless
	// of --yes (D-02; asserted by TestWriteVerbsConfirmationFlagStillBacksUp).
	res, derr := cliDeleteInto(b, name, scope, policy)
	if derr != nil {
		if len(res.Restored) > 0 {
			fmt.Fprintf(cmd.ErrOrStderr(), "restored: %s\n", strings.Join(displayMessages(b, res.Restored), "; ")) //nolint:errcheck // best-effort diagnostic
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

// printDeleteDryRun prints the full DeletePlan for name/scope and exits
// without writing (--dry-run). It runs NO connectivity test — delete's row in
// plan 05-07's lifecycleStages has no test stage (asserted by a recording
// seam-count test) — and it is a real error when the identity is unknown.
func printDeleteDryRun(w io.Writer, b *realBackend, name string, scope identity.DeleteScope) error {
	plan, err := b.DeletePlan(name, string(scope))
	if err != nil {
		return err
	}
	return renderDeletePlan(w, b, plan, "would delete")
}

// renderDeletePlan renders the DeletePlan a confirm screen or --dry-run
// shows: the target list (with block/label), the shared-key downgrade note,
// the unmanaged-reference scan hits, the everything-scope key-copy archive
// path, and the disclaimer. verbLabel is "would delete" for a dry run and
// "will delete" for a real run's disclosure.
func renderDeletePlan(w io.Writer, b *realBackend, plan tuikit.DeletePlanView, verbLabel string) error {
	if _, err := fmt.Fprintf(w, "%s %q (%s)\n", verbLabel, plan.Name, plan.Scope); err != nil {
		return err
	}
	for _, t := range plan.Targets {
		label := t.Label
		if label == "" {
			label = t.Block
		}
		if _, err := fmt.Fprintf(w, "  target: %s — %s\n", b.displayPath(t.File), label); err != nil {
			return err
		}
	}
	if plan.ProviderRewriteTarget != nil {
		if _, err := fmt.Fprintf(w, "  provider rewrite: %s (removed when the last identity for this provider goes)\n", b.displayPath(plan.ProviderRewriteTarget.File)); err != nil {
			return err
		}
	}
	if len(plan.SharedKeyOwners) > 0 {
		if _, err := fmt.Fprintf(w, "  shared key: also used by %s — the key pair is KEPT for them\n", strings.Join(plan.SharedKeyOwners, ", ")); err != nil {
			return err
		}
	}
	for _, h := range plan.Hits {
		if _, err := fmt.Fprintf(w, "  unmanaged reference: %s:%d (%s) — %s\n", b.displayPath(h.File), h.Line, h.Region, h.Text); err != nil {
			return err
		}
	}
	if plan.KeyCopyPath != "" {
		if _, err := fmt.Fprintf(w, "  key pair: copied to %s before removal\n", plan.KeyCopyPath); err != nil {
			return err
		}
	}
	if plan.Disclaimer != "" {
		if _, err := fmt.Fprintf(w, "  %s\n", plan.Disclaimer); err != nil {
			return err
		}
	}
	return nil
}
