package main

// identity_key.go implements `gitid identity rotate <name>` and
// `gitid identity new-key <name>` — the two D-02 key-lifecycle verbs. Both are
// thin adapters over plan 05-07's lifecycle chokepoints: rotate calls
// b.runRotate, new-key calls b.runRepair, each with a lifecyclePolicy built
// from the flags (review R-11-CLI). Neither reimplements a stage. The
// recording double tests override cliRotateInto / cliRepairInto to prove the
// handler invokes the lifecycle exactly once and performs no stage of its
// own.

import (
	"bufio"
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/castocolina/gitid/internal/identity"
	"github.com/castocolina/gitid/internal/sshconfig"
	"github.com/castocolina/gitid/internal/tester"
)

// identityKeyFlags carries the rotate/new-key flags as a plain struct.
type identityKeyFlags struct {
	Yes      bool
	DryRun   bool
	NoUpload bool
}

// newIdentityRotateVerb builds the `rotate <name>` verb spec.
func newIdentityRotateVerb() identityVerb {
	var flags identityKeyFlags
	return identityVerb{
		use:     "rotate <name>",
		aliases: []string{"rotate-key"},
		short:   "Rotate an identity's key: archive the current pair and generate a new one at the same canonical paths",
		args:    cobra.ExactArgs(1),
		bindFlags: func(fs *pflag.FlagSet) {
			fs.BoolVar(&flags.Yes, "yes", false, "skip the confirmation prompt; the timestamped backup and the key archive are still taken unconditionally")
			fs.BoolVar(&flags.DryRun, "dry-run", false, dryRunKeyVerbFlagHelp)
			fs.BoolVar(&flags.NoUpload, "no-upload", false, noUploadFlagHelp)
		},
		run: func(cmd *cobra.Command, args []string) error {
			return runIdentityKeyVerb(cmd, args[0], "rotate", flags, termIsStdinTTY(), termIsStdoutTTY())
		},
	}
}

// newIdentityNewKeyVerb builds the `new-key <name>` verb spec.
func newIdentityNewKeyVerb() identityVerb {
	var flags identityKeyFlags
	return identityVerb{
		use:     "new-key <name>",
		aliases: []string{"repair"},
		short:   "Generate a fresh key for an identity (repair: never touches pre-existing key material)",
		args:    cobra.ExactArgs(1),
		bindFlags: func(fs *pflag.FlagSet) {
			fs.BoolVar(&flags.Yes, "yes", false, "skip the confirmation prompt; the timestamped backup is still taken unconditionally")
			fs.BoolVar(&flags.DryRun, "dry-run", false, dryRunKeyVerbFlagHelp)
			fs.BoolVar(&flags.NoUpload, "no-upload", false, noUploadFlagHelp)
		},
		run: func(cmd *cobra.Command, args []string) error {
			return runIdentityKeyVerb(cmd, args[0], "new-key", flags, termIsStdinTTY(), termIsStdoutTTY())
		},
	}
}

// cliRotateInto / cliRepairInto are the CLI's lifecycle seams: thin
// adapters over the SAME runRotate / runRepair the TUI's commit seams call.
// test-only vars so a recording double can assert the handler invokes the
// lifecycle exactly once.
var (
	cliRotateInto = func(b *realBackend, name string, p lifecyclePolicy) (lifecycleResult, error) {
		return b.runRotate(name, p)
	}
	cliRepairInto = func(b *realBackend, name string, p lifecyclePolicy) (lifecycleResult, error) {
		return b.runRepair(name, p)
	}
	// cliConnectivityTest is the ONLY connectivity-tester seam the CLI key
	// verbs reach (the rotate/new-key dry run's current-key probe). A
	// recording double over it proves delete never invokes the tester.
	cliConnectivityTest = func(b *realBackend, alias string) tester.Result {
		res, _ := b.deps.Resolved(alias)
		return res
	}
)

// rotateDryRunCaveat is the FROZEN post-rotation caveat sentence the rotate/
// new-key dry-run prints and the parity matrix row carries (review R2-13).
// It is registered in Makefile gate-copy-freeze; reword it and the gate fails.
const rotateDryRunCaveat = "This dry run tests only the current key's reachability — the new key has not been generated, uploaded, or resolved, so nothing about the post-rotation state is proven."

// runIdentityKeyVerb drives both key verbs through their shared D-02 flow: a
// dry run prints the plan and probes the CURRENT key without generating
// anything; a real run maps the flags onto the confirmation enum and calls
// the ONE lifecycle function, driving the exit code from the post-write
// re-test landing state.
func runIdentityKeyVerb(cmd *cobra.Command, name, verb string, flags identityKeyFlags, stdinTTY, stdoutTTY bool) error {
	if flags.DryRun {
		home, err := resolveHomeForCLI()
		if err != nil {
			return err
		}
		b := newBackendForHome(home)
		return printKeyCeremonyDryRun(cmd.OutOrStdout(), b, name, verb, flags.NoUpload)
	}

	// The authorization gate runs BEFORE a backend is even built or a policy
	// constructed (review R2-03): a non-interactive destructive invocation
	// without --yes is refused here, and the regression test proves no
	// lifecycle function is invoked in that case.
	if !flags.Yes && (!stdinTTY || !stdoutTTY) {
		return fmt.Errorf("gitid: refusing to %s %q without --yes in non-interactive mode (use --yes to skip only the confirmation prompt; the timestamped backup is still taken)", verb, name)
	}

	home, err := resolveHomeForCLI()
	if err != nil {
		return err
	}
	b := newBackendForHome(home)

	policy, perr := confirmationPolicyFrom(cmd, verb+" "+name, stdinTTY, stdoutTTY, flags.Yes, func(string) (bool, error) {
		fmt.Fprintf(cmd.OutOrStdout(), "%s identity %q? Type \"yes\" to confirm: ", keyVerbLabel(verb), name) //nolint:errcheck // best-effort prompt
		reader := bufio.NewReader(cmd.InOrStdin())
		line, _ := reader.ReadString('\n')
		return strings.TrimSpace(line) == "yes", nil
	})
	if perr != nil {
		return perr
	}

	var res lifecycleResult
	var lerr error
	if verb == "new-key" {
		res, lerr = cliRepairInto(b, name, policy)
	} else {
		res, lerr = cliRotateInto(b, name, policy)
	}
	if lerr != nil {
		if len(res.Restored) > 0 {
			fmt.Fprintf(cmd.ErrOrStderr(), "restored: %s\n", strings.Join(displayMessages(b, res.Restored), "; ")) //nolint:errcheck // best-effort diagnostic
		}
		return lerr
	}
	for _, bak := range res.Backups {
		fmt.Fprintf(cmd.OutOrStdout(), "backed up -> %s\n", b.displayPath(bak)) //nolint:errcheck // best-effort stdout
	}
	for _, archived := range res.ArchivedKeyPaths {
		fmt.Fprintf(cmd.OutOrStdout(), "old key archived -> %s\n", b.displayPath(archived)) //nolint:errcheck // best-effort stdout
	}
	fmt.Fprintf(cmd.OutOrStdout(), "%s took effect for %q\n", verb, name) //nolint:errcheck // best-effort stdout

	// D-05/D-03/D-11: the upload step runs AFTER the key material exists
	// (the lifecycle write above just committed it) and BEFORE the
	// post-write re-test below — mirroring the wizard's own ordering (D-05
	// places upload before the test loop so a fresh key can genuinely pass
	// on the first probe) — and it never alters this function's control
	// flow or its returned error: the exit code is decided solely by the
	// re-test outcome, exactly as it would be with the upload disabled.
	if acct, found := b.findAccount(name); found {
		runUploadStep(cmd.OutOrStdout(), b, uploadRequestForAccount(acct, b.home), flags.NoUpload, false)
	}

	// The process exit code is driven by the post-write re-test outcome
	// (D-02): a hard Failure names the outcome and exits non-zero; PASS and
	// reachable-not-uploaded are both accepted landing states (a fresh key is
	// not expected to be pre-registered with the provider), so they exit
	// zero.
	if res.ReTest.Outcome == tester.Failure {
		return fmt.Errorf("gitid: %s wrote, but the post-write connectivity re-test failed (outcome: %s) — the provider could not be reached through the new configuration", verb, outcomeLabel(res.ReTest.Outcome))
	}
	return nil
}

// printKeyCeremonyDryRun prints the rotate/new-key ceremony plan (targets,
// archive destination shape for rotate, provider host) and, when the identity
// currently has a key, runs the connectivity probe against that CURRENT key
// (review R2-13). It generates NO key material, stages nothing, archives
// nothing, and writes nothing. The output names what was actually tested and
// carries the frozen caveat sentence.
func printKeyCeremonyDryRun(w io.Writer, b *realBackend, name, verb string, noUpload bool) error {
	acct, found := b.findAccount(name)
	if !found {
		return fmt.Errorf("gitid: no such identity: %q", name)
	}
	acct = b.normalizeAccountForWrite(acct)

	fmt.Fprintf(w, "dry run: would %s %q\n", verb, name) //nolint:errcheck // best-effort stdout
	fmt.Fprintf(w, "  provider: %s\n", acct.Provider)    //nolint:errcheck // best-effort stdout
	privTarget := acct.KeyPath
	if verb == "new-key" {
		if p, _ := identity.RepairKeyPath(acct); p != "" {
			privTarget = p
		}
	}
	fmt.Fprintf(w, "  key: %s\n", b.displayPath(privTarget))                              //nolint:errcheck // best-effort stdout
	fmt.Fprintf(w, "  targets: %s\n", strings.Join(b.keyDryRunTargets(acct, verb), ", ")) //nolint:errcheck // best-effort stdout
	if verb == "rotate" {
		fmt.Fprintf(w, "  archive: %s (D-06 archive directory shape)\n", b.displayPath(sshconfig.ArchiveDir(b.sshDir))) //nolint:errcheck // best-effort stdout
	}
	if fileExists(acct.KeyPath) {
		res := cliConnectivityTest(b, acct.Alias)
		fmt.Fprintf(w, "  connectivity test (CURRENT key %s): %s\n", b.displayPath(acct.KeyPath), outcomeLabel(res.Outcome)) //nolint:errcheck // best-effort stdout
	} else {
		fmt.Fprintf(w, "  connectivity test: skipped (no current key present)\n") //nolint:errcheck // best-effort stdout
	}
	// R2-13: the caveat is part of the output, not just the docs.
	fmt.Fprintf(w, "  %s\n", rotateDryRunCaveat) //nolint:errcheck // best-effort stdout

	// R11/D-06: the upload preview is ADDITIONAL dry-run output. rotate/
	// new-key's dry run generates no new key, so the preview is against the
	// identity's CURRENT public key (the only real .pub file that exists
	// pre-write) when one is present.
	if fileExists(acct.PubPath) {
		runUploadStep(w, b, uploadRequestForAccount(acct, b.home), noUpload, true)
	}
	return nil
}

// keyDryRunTargets lists the managed files a rotate/new-key write would touch.
func (b *realBackend) keyDryRunTargets(acct identity.Account, verb string) []string {
	targets := []string{b.displayPath(b.storageTargetPath()), b.displayPath(b.gitconfigPath), b.displayPath(b.allowedSigners)}
	if acct.FragmentPath != "" {
		targets = append(targets, b.displayPath(acct.FragmentPath))
	}
	if verb == "rotate" {
		if acct.KeyPath != "" {
			targets = append(targets, b.displayPath(acct.KeyPath), b.displayPath(acct.PubPath))
		}
	} else if priv, pub := identity.RepairKeyPath(acct); priv != "" {
		targets = append(targets, b.displayPath(priv), b.displayPath(pub))
	}
	return targets
}

// keyVerbLabel returns the human-facing verb label for the confirmation
// prompt ("Rotate"/"New key").
func keyVerbLabel(verb string) string {
	if verb == "new-key" {
		return "New key for"
	}
	if len(verb) == 0 {
		return verb
	}
	return strings.ToUpper(verb[:1]) + verb[1:]
}
