package main

// identity_upload.go implements 09-05-PLAN.md's CLI upload surface:
//
//   - Task 2: `gitid identity register-key <name>` / `gitid register-key
//     <name>` — the manual re-run surface (D-01's substance, since the
//     archived `gitid copy --upload-keys` command no longer exists —
//     09-05-PLAN.md's objective documents this scoped deviation from
//     09-CONTEXT.md's stale phrasing). Its exit contract (R10) differs
//     deliberately from the four write verbs: register-key's registration
//     IS the primary operation, so it reports total failure in its exit
//     code — a headless surface that always exits 0 tells a script
//     nothing.
//   - Task 3: the shared `--no-upload` help text every write verb binds
//     identically.

import (
	"fmt"
	"io"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/castocolina/gitid/internal/tuikit"
)

// noUploadFlagHelp is declared ONCE and referenced by every one of create,
// clone, rotate, and new-key's bindFlags (09-05-PLAN.md Task 3) so the four
// bindings cannot drift — TestNoUploadFlagIsBoundOnAllFourWriteVerbs asserts
// byte-identical usage text across all four.
const noUploadFlagHelp = "skip the autonomous provider key registration this operation would otherwise attempt; the primary operation still runs and its exit code is unaffected either way"

// identityRegisterKeyFlags carries the register-key verb's flags.
type identityRegisterKeyFlags struct {
	DryRun bool
}

// newIdentityRegisterKeyVerb builds the `register-key [name]` verb spec.
func newIdentityRegisterKeyVerb() identityVerb {
	var flags identityRegisterKeyFlags
	return identityVerb{
		use:     "register-key [name]",
		aliases: []string{"register"},
		short:   "Register an identity's public key with its provider for authentication and signing",
		args:    cobra.MaximumNArgs(1),
		bindFlags: func(fs *pflag.FlagSet) {
			// R11: precise about scope — a dry run performs no remote
			// MUTATION, but the authentication status check and the key
			// inventory read used to compute the missing-type diff may
			// still run. "executes nothing" would promise more than the
			// behavior delivers.
			fs.BoolVar(&flags.DryRun, "dry-run", false, "print the exact commands about to run and perform no remote MUTATION; the auth-status check and the key-inventory read (used to compute the missing-type diff) may still run as read-only probes")
		},
		run: func(cmd *cobra.Command, args []string) error {
			return runIdentityRegisterKey(cmd, args, flags, termIsStdinTTY(), termIsStdoutTTY())
		},
	}
}

// cliRegisterKeyInto is the register-key lifecycle seam: the ONE upload
// orchestration entry point this verb calls, defaulting to the SAME
// runUploadFor the derived-autonomous-upload step on create/clone/rotate/
// new-key calls. test-only var so a recording double can assert the
// handler invokes the orchestration exactly once and performs no upload
// step of its own.
var cliRegisterKeyInto = func(b *realBackend, req uploadRequest) tuikit.UploadRunView {
	return b.runUploadFor(req)
}

// runIdentityRegisterKey is register-key's D-02 adaptive-depth entry
// point: a missing name routes through the SAME depthResolver every other
// write verb uses (09-RESEARCH.md Pattern 5 — a bespoke TTY check here
// would make this the one verb with different edge-case behavior).
func runIdentityRegisterKey(cmd *cobra.Command, args []string, flags identityRegisterKeyFlags, stdinTTY, stdoutTTY bool) error {
	var name string
	if len(args) > 0 {
		name = args[0]
	}
	res, missing := depthResolver{
		required:  []string{"name"},
		supplied:  map[string]bool{"name": name != ""},
		stdinTTY:  stdinTTY,
		stdoutTTY: stdoutTTY,
	}.resolve()

	switch res {
	case resolvePrefilledTUI:
		home, err := resolveHomeForCLI()
		if err != nil {
			return err
		}
		b := newBackendForHome(home)
		return runPrefilledTUI(b, nil)
	case resolveMissingFlags:
		return missingFlagErr("register-key", missing)
	case resolveHeadless:
	}

	home, err := resolveHomeForCLI()
	if err != nil {
		return err
	}
	b := newBackendForHome(home)
	acct, found := b.findAccount(name)
	if !found {
		return fmt.Errorf("gitid: no such identity: %q", name)
	}
	req := uploadRequestForAccount(acct, b.home)

	if flags.DryRun {
		plan, terminal := b.planUpload(req)
		if terminal != nil {
			printUploadOutcome(cmd.OutOrStdout(), *terminal)
			return nil
		}
		for _, c := range plan.commands {
			fmt.Fprintln(cmd.OutOrStdout(), fmt.Sprintf(tuikit.UploadRunningLineFmt, c)) //nolint:errcheck // best-effort stdout
		}
		fmt.Fprintln(cmd.OutOrStdout(), tuikit.UploadDryRunNote) //nolint:errcheck // best-effort stdout
		return nil
	}

	view := cliRegisterKeyInto(b, req)
	printUploadOutcome(cmd.OutOrStdout(), view)

	// R10: register-key's registration IS the primary operation, so —
	// unlike the four write verbs, where D-03 forbids an upload failure
	// from changing the PRIMARY operation's exit code — total registration
	// failure here IS the primary operation failing, and reporting it is
	// D-03 applied rather than D-03 broken. A headless surface that always
	// exits 0 cannot be scripted against. Non-zero only when at least one
	// registration was ATTEMPTED (row.Command != "" — set only when
	// requirePublicKey passed and buildArgs ran, i.e. a real ssh-key add
	// attempt was made) and every attempted registration failed; the full
	// outcome (including the manual-fallback block) was already printed
	// above regardless of the exit code.
	attempted, failed := 0, 0
	for _, row := range view.Rows {
		if row.Command == "" {
			continue
		}
		attempted++
		if row.Outcome == tuikit.UploadRowFailed {
			failed++
		}
	}
	if attempted > 0 && attempted == failed {
		return fmt.Errorf("gitid: register-key %q: every attempted registration failed", name)
	}
	return nil
}

// runUploadStep attaches the derived-autonomous-upload behavior 09-05-
// PLAN.md's Task 3 adds to create, clone, rotate, and new-key: without
// --no-upload it calls runUploadFor and prints the outcome; with
// --no-upload it prints the skip note and the manual-fallback block and
// performs zero provider-CLI invocations; in a dry run it prints the
// command previews and the dry-run note and executes nothing. It NEVER
// alters the caller's control flow — callers ignore the return value by
// design, since D-03/D-11 forbid an upload outcome from changing the
// exit code of these four PRIMARY operations. A panic cannot escape
// (runUploadFor already recovers internally), so this call is safe to make
// unconditionally once the key material exists.
func runUploadStep(w io.Writer, b *realBackend, req uploadRequest, noUpload, dryRun bool) {
	if noUpload {
		// WR-02: route through printUploadOutcome (SkippedByFlag=true) rather
		// than printing directly, so the --no-upload note has exactly ONE
		// rendering site — the same site planUpload's Skipped-but-not-by-flag
		// states (Omitted/Disabled) deliberately do NOT trigger it.
		view := tuikit.UploadRunView{SkippedByFlag: true, ManualFallback: b.UploadInstructions(req.Hostname)}
		printUploadOutcome(w, view)
		return
	}
	if dryRun {
		plan, terminal := b.planUpload(req)
		if terminal != nil {
			printUploadOutcome(w, *terminal)
			return
		}
		for _, c := range plan.commands {
			fmt.Fprintln(w, fmt.Sprintf(tuikit.UploadRunningLineFmt, c)) //nolint:errcheck // best-effort stdout
		}
		fmt.Fprintln(w, tuikit.UploadDryRunNote) //nolint:errcheck // best-effort stdout
		return
	}
	view := b.runUploadFor(req)
	printUploadOutcome(w, view)
}
