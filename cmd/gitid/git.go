package main

// git.go is the real `gitid git` command group (plan 07-05): options list /
// options apply / fallback show / fallback set. Both write verbs call the
// SAME UI-free ceremonies the TUI uses — runGlobalGitApply and
// runGitFallbackAuthorApply — never a tea.Cmd and never a CLI-only write path.

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"
	"text/tabwriter"

	tea "charm.land/bubbletea/v2"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"golang.org/x/term"

	"github.com/castocolina/gitid/internal/globalgit"
	"github.com/castocolina/gitid/internal/tuikit"
)

const (
	gitApplyRequiredKey = "<key>"
	gitFallbackRequired = "--name/--email/--clear-name/--clear-email"

	// The four frozen schema identifiers (07-05 <authority>). A consumer may
	// rely on each key set staying byte-identical until the identifier bumps.
	gitOptionsSchema     = "gitid.git.options/v1"
	gitApplySchema       = "gitid.git.apply/v1"
	gitFallbackSchema    = "gitid.git.fallback/v1"
	gitFallbackSetSchema = "gitid.git.fallbackset/v1"

	// gitStateNeedsAction..gitStateProbeError are the JSON state taxonomy,
	// shared with the render layer's own vocabulary: every state the Options
	// pane and the refusals display is one of these five strings.
	gitStateNeedsAction   = "needs-action"
	gitStateAlreadySet    = "already-set"
	gitStateDiffers       = "differs"
	gitStateNotApplicable = "not-applicable"
	gitStateProbeError    = "probe-error"

	gitStatusSet   = "set"
	gitStatusUnset = "unset"
)

// cliGlobalGitApplyInto / cliGitFallbackAuthorApplyInto are the CLI's
// lifecycle seams: thin adapters over the SAME runGlobalGitApply /
// runGitFallbackAuthorApply the TUI's commit seams call. Test-only vars so
// a recording double can assert the handler invokes the ceremony exactly once.
var (
	cliGlobalGitApplyInto = func(b *realBackend, keys []string, p lifecyclePolicy) (lifecycleResult, error) {
		return b.runGlobalGitApply(keys, p)
	}
	cliGitFallbackAuthorApplyInto = func(b *realBackend, name, email string, p lifecyclePolicy) (lifecycleResult, error) {
		return b.runGitFallbackAuthorApply(name, email, p)
	}
	gitTUILaunch = func(b *realBackend) error {
		_, err := tea.NewProgram(tuikit.NewAppOnGlobalGit(b)).Run()
		return err
	}
	cliGitOptionStates = func(b *realBackend) ([]tuikit.GlobalGitOptionView, error) {
		return b.GlobalGitOptionStates()
	}
)

// gitWriteExitCode maps a lifecycleResult plus its error plus the
// --fail-on-advisory opt-in onto the SAME frozen exit-status table as the SSH
// noun (07-05 <authority>): 0 success even with advisories and always for a
// dry run, 1 refusal, 2 a rolled-back write, 3 the advisory opt-in on a
// successful advisory-carrying write. One helper produces BOTH the process
// status and the envelope's exit_code field, so a script that trusts the exit
// status and a consumer that reads the envelope can never disagree.
func gitWriteExitCode(res lifecycleResult, err error, failOnAdvisory, dryRun bool) int {
	return sshWriteExitCode(res, err, failOnAdvisory, dryRun)
}

func newGitCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "git",
		Short: "Manage global Git options and the fallback author",
	}
	cmd.AddCommand(newGitOptionsCmd())
	cmd.AddCommand(newGitFallbackCmd())
	return cmd
}

func newGitOptionsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "options",
		Short: "List or apply global Git options",
	}
	cmd.AddCommand(newVerbCmd(newGitOptionsListVerb()))
	cmd.AddCommand(newVerbCmd(newGitOptionsApplyVerb()))
	return cmd
}

func newGitFallbackCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "fallback",
		Short: "Show or set the global fallback author pair",
	}
	cmd.AddCommand(newVerbCmd(newGitFallbackShowVerb()))
	cmd.AddCommand(newVerbCmd(newGitFallbackSetVerb()))
	return cmd
}

func gitApplyTokens() []string {
	out := make([]string, 0, len(globalgit.Policy))
	for _, p := range globalgit.Policy {
		if p.Token != "" {
			out = append(out, p.Token)
		}
	}
	return out
}

func gitApplyTokenHelp() string {
	return "Accepted tokens (exact, case-sensitive): " + strings.Join(gitApplyTokens(), ", ") +
		". Member config keys (for example alias.lg or core.eol) are refused; type the row token instead."
}

func newGitOptionsListVerb() identityVerb {
	var jsonOut bool
	return identityVerb{
		use:   "list",
		short: "List the global Git options with current value, provenance, recommended value and state",
		args:  cobra.NoArgs,
		bindFlags: func(fs *pflag.FlagSet) {
			fs.BoolVar(&jsonOut, "json", false, "print the frozen gitid.git.options/v1 document")
		},
		run: func(cmd *cobra.Command, _ []string) error {
			home, err := resolveHomeForCLI()
			if err != nil {
				return err
			}
			b := newBackendForHome(home)
			recs, err := gitOptionRecords(b)
			if err != nil {
				return err
			}
			isTTY := term.IsTerminal(int(os.Stdout.Fd()))
			return renderGitOptionsList(cmd.OutOrStdout(), isTTY, jsonOut, recs)
		},
	}
}

type gitApplyFlags struct {
	Yes            bool
	DryRun         bool
	FailOnAdvisory bool
	JSON           bool
}

func newGitOptionsApplyVerb() identityVerb {
	var flags gitApplyFlags
	return identityVerb{
		use:   "apply [key...]",
		short: "Apply named global Git options toward their recommended values. " + gitApplyTokenHelp(),
		args:  cobra.ArbitraryArgs,
		bindFlags: func(fs *pflag.FlagSet) {
			fs.BoolVar(&flags.Yes, "yes", false, "skip the confirmation prompt; the timestamped backup is still taken unconditionally")
			fs.BoolVar(&flags.DryRun, "dry-run", false, "print the plan and exit 0 without writing")
			fs.BoolVar(&flags.FailOnAdvisory, "fail-on-advisory", false, "exit 3 when a successful write reports an advisory (default: exit 0)")
			fs.BoolVar(&flags.JSON, "json", false, "print the frozen gitid.git.apply/v1 write-result document")
		},
		run: func(cmd *cobra.Command, args []string) error {
			return runGitOptionsApply(cmd, args, flags, termIsStdinTTY(), termIsStdoutTTY())
		},
	}
}

func newGitFallbackShowVerb() identityVerb {
	var jsonOut bool
	return identityVerb{
		use:   "show",
		short: "Show the global fallback author pair",
		args:  cobra.NoArgs,
		bindFlags: func(fs *pflag.FlagSet) {
			fs.BoolVar(&jsonOut, "json", false, "print the frozen gitid.git.fallback/v1 document")
		},
		run: func(cmd *cobra.Command, _ []string) error {
			return runGitFallbackShow(cmd, jsonOut)
		},
	}
}

type gitFallbackSetFlags struct {
	Name        string
	Email       string
	ClearName   bool
	ClearEmail  bool
	Yes         bool
	DryRun      bool
	JSON        bool
	nameWasSet  bool
	emailWasSet bool
}

func newGitFallbackSetVerb() identityVerb {
	var flags gitFallbackSetFlags
	return identityVerb{
		use: "set",
		short: "Set or clear halves of the global fallback author. " +
			"--name/--email set a half; --clear-name/--clear-email unset it; omitting a flag leaves that half unchanged. " +
			"An empty value is a refusal, not a silent clear. A script that forgets a flag must never erase the user's identity.",
		args: cobra.NoArgs,
		bindFlags: func(fs *pflag.FlagSet) {
			fs.StringVar(&flags.Name, "name", "", "set the fallback user.name; empty is refused, not a clear")
			fs.StringVar(&flags.Email, "email", "", "set the fallback user.email; empty is refused, not a clear")
			fs.BoolVar(&flags.ClearName, "clear-name", false, "unset the fallback user.name")
			fs.BoolVar(&flags.ClearEmail, "clear-email", false, "unset the fallback user.email")
			fs.BoolVar(&flags.Yes, "yes", false, "skip the confirmation prompt; the timestamped backup is still taken unconditionally")
			fs.BoolVar(&flags.DryRun, "dry-run", false, "print the plan and exit 0 without writing")
			fs.BoolVar(&flags.JSON, "json", false, "print the frozen gitid.git.fallbackset/v1 write-result document")
		},
		run: func(cmd *cobra.Command, _ []string) error {
			flags.nameWasSet = cmd.Flags().Changed("name")
			flags.emailWasSet = cmd.Flags().Changed("email")
			return runGitFallbackSet(cmd, flags, termIsStdinTTY(), termIsStdoutTTY())
		},
	}
}

func runGitOptionsApply(cmd *cobra.Command, tokens []string, flags gitApplyFlags, stdinTTY, stdoutTTY bool) error {
	supplied := map[string]bool{gitApplyRequiredKey: len(tokens) > 0}
	outcome, missing := depthResolver{
		required:  []string{gitApplyRequiredKey},
		supplied:  supplied,
		stdinTTY:  stdinTTY,
		stdoutTTY: stdoutTTY,
	}.resolve()
	switch outcome {
	case resolvePrefilledTUI:
		home, err := resolveHomeForCLI()
		if err != nil {
			return err
		}
		return gitTUILaunch(newBackendForHome(home))
	case resolveMissingFlags:
		err := missingFlagErr("git options apply", missing)
		if flags.JSON {
			env := newGitApplyEnvelope(flags.DryRun)
			env.Error = err.Error()
			env.ExitCode = 1
			return finishGitApply(cmd, true, env, err)
		}
		return sshFinish(1, err)
	case resolveHeadless:
	}

	home, err := resolveHomeForCLI()
	if err != nil {
		return sshFinish(1, err)
	}
	b := newBackendForHome(home)

	env := newGitApplyEnvelope(flags.DryRun)
	env.TargetPath = b.displayPath(b.baselineTargetPath())

	keys, verr := validateGitApplyTokens(b, tokens)
	if verr != nil {
		env.Declined = append([]string{}, tokens...)
		env.Applied = []string{}
		env.Error = verr.Error()
		env.ExitCode = 1
		if flags.JSON {
			return finishGitApply(cmd, true, env, verr)
		}
		return sshFinish(1, verr)
	}

	if flags.DryRun {
		res, rerr := cliGlobalGitApplyInto(b, keys, lifecyclePolicy{DryRun: true})
		if !flags.JSON {
			plan, _ := b.GlobalGitApplyPlan(keys)
			printGitApplyDryRun(cmd.OutOrStdout(), plan)
		}
		fillGitApplyFromResult(b, &env, res, rerr, tokens)
		env.ExitCode = gitWriteExitCode(res, rerr, flags.FailOnAdvisory, true)
		return finishGitApply(cmd, flags.JSON, env, rerr)
	}

	policy, perr := confirmationPolicyFrom(cmd, "apply global Git options", stdinTTY, stdoutTTY, flags.Yes, func(preview string) (bool, error) {
		if plan, perr := b.GlobalGitApplyPlan(keys); perr == nil {
			printGitApplyDryRun(cmd.OutOrStdout(), plan)
		}
		fmt.Fprintf(cmd.OutOrStdout(), "%s\nType \"yes\" to confirm: ", preview) //nolint:errcheck
		reader := bufio.NewReader(cmd.InOrStdin())
		line, _ := reader.ReadString('\n')
		return strings.TrimSpace(line) == "yes", nil
	})
	if perr != nil {
		env.Declined = append([]string{}, tokens...)
		env.Applied = []string{}
		env.Error = perr.Error()
		env.ExitCode = 1
		return finishGitApply(cmd, flags.JSON, env, perr)
	}

	res, rerr := cliGlobalGitApplyInto(b, keys, policy)
	fillGitApplyFromResult(b, &env, res, rerr, tokens)
	env.ExitCode = gitWriteExitCode(res, rerr, flags.FailOnAdvisory, false)
	if !flags.JSON {
		printGitApplyHuman(cmd, b, res, rerr, tokens)
	}
	return finishGitApply(cmd, flags.JSON, env, rerr)
}

// validateGitApplyTokens validates every argv token in two passes so that an
// unknown token or a member-config key is refused by name BEFORE anything is
// read or written (plan 07-05's own requirement) — the first pass is a pure
// static lookup against internal/globalgit's frozen token table, with no I/O
// at all, so it cannot be preempted by an unrelated probe failure. Only once
// every token resolves to a real row does the second pass call
// cliGitOptionStates (which DOES require a probe) to check selectability.
func validateGitApplyTokens(b *realBackend, tokens []string) ([]string, error) {
	policies := make([]globalgit.OptionPolicy, 0, len(tokens))
	for _, tok := range tokens {
		policy, ok := globalgit.PolicyForToken(tok)
		if !ok {
			if owner, found := globalgit.TokenOwningMember(tok); found {
				return nil, fmt.Errorf("gitid: %q is a member config key; use the row token %q instead", tok, owner)
			}
			return nil, fmt.Errorf("gitid: unknown global git option %q", tok)
		}
		policies = append(policies, policy)
	}

	views, verr := cliGitOptionStates(b)
	if verr != nil {
		return nil, verr
	}
	byKey := make(map[string]tuikit.GlobalGitOptionView, len(views))
	for _, v := range views {
		byKey[v.Key] = v
	}
	keys := make([]string, 0, len(tokens))
	for i, policy := range policies {
		view, has := byKey[policy.Key]
		if !has {
			// Fail closed (code review finding): a policy key genuinely
			// missing from the live probe results has no real
			// classification to trust. ProbeError set here makes
			// Selectable() return false rather than defaulting to the
			// zero-value NeedsAction state, which would otherwise accept
			// a token this CLI has no actual evidence for. Currently
			// unreachable — globalgit.Statuses always returns exactly one
			// row per Policy entry — but this keeps that invariant from
			// silently becoming a selectable-by-default trap if it's ever
			// broken by a future partial-probe optimization.
			view = tuikit.GlobalGitOptionView{Key: policy.Key, PolicyBacked: true, HasWritableMember: len(policy.Members) > 0, ProbeError: "row missing from probe results"}
		}
		if !view.Selectable() {
			return nil, fmt.Errorf("gitid: refusing to apply %q: row is %s", tokens[i], gitRowStateName(view))
		}
		keys = append(keys, policy.Key)
	}
	return keys, nil
}

func gitRowStateName(v tuikit.GlobalGitOptionView) string {
	if v.ProbeError != "" {
		return gitStateProbeError
	}
	switch v.State {
	case tuikit.GlobalGitAlreadySet:
		return gitStateAlreadySet
	case tuikit.GlobalGitSetButDiffers:
		return gitStateDiffers
	case tuikit.GlobalGitNotApplicable:
		return gitStateNotApplicable
	default:
		return gitStateNeedsAction
	}
}

func runGitFallbackShow(cmd *cobra.Command, jsonOut bool) error {
	home, err := resolveHomeForCLI()
	if err != nil {
		return err
	}
	b := newBackendForHome(home)
	state, serr := b.GitFallbackAuthorState()
	if serr != nil {
		return sshFinish(1, fmt.Errorf("gitid: cannot read fallback author from %s: %w", b.displayPath(b.gitconfigPath), serr))
	}
	nameStatus, emailStatus := gitStatusUnset, gitStatusUnset
	if state.Name != "" {
		nameStatus = gitStatusSet
	}
	if state.Email != "" {
		emailStatus = gitStatusSet
	}
	if jsonOut {
		return writeJSON(cmd.OutOrStdout(), gitFallbackDocument{
			Schema:      gitFallbackSchema,
			Name:        state.Name,
			Email:       state.Email,
			NameStatus:  nameStatus,
			EmailStatus: emailStatus,
		})
	}
	_, err = fmt.Fprintf(cmd.OutOrStdout(), "name\t%s\t%s\nemail\t%s\t%s\n",
		nameStatus, state.Name, emailStatus, state.Email)
	return err
}

func runGitFallbackSet(cmd *cobra.Command, flags gitFallbackSetFlags, stdinTTY, stdoutTTY bool) error {
	anyFlag := flags.nameWasSet || flags.emailWasSet || flags.ClearName || flags.ClearEmail
	supplied := map[string]bool{gitFallbackRequired: anyFlag}
	outcome, missing := depthResolver{
		required:  []string{gitFallbackRequired},
		supplied:  supplied,
		stdinTTY:  stdinTTY,
		stdoutTTY: stdoutTTY,
	}.resolve()
	switch outcome {
	case resolvePrefilledTUI:
		home, err := resolveHomeForCLI()
		if err != nil {
			return err
		}
		return gitTUILaunch(newBackendForHome(home))
	case resolveMissingFlags:
		err := missingFlagErr("git fallback set", missing)
		if flags.JSON {
			env := newGitFallbackSetEnvelope(flags.DryRun)
			env.Error = err.Error()
			env.ExitCode = 1
			return finishGitFallbackSet(cmd, true, env, err)
		}
		return sshFinish(1, err)
	case resolveHeadless:
	}

	if verr := validateFallbackSetFlags(flags); verr != nil {
		if flags.JSON {
			env := newGitFallbackSetEnvelope(flags.DryRun)
			env.Error = verr.Error()
			env.ExitCode = 1
			return finishGitFallbackSet(cmd, true, env, verr)
		}
		return sshFinish(1, verr)
	}

	home, err := resolveHomeForCLI()
	if err != nil {
		return sshFinish(1, err)
	}
	b := newBackendForHome(home)
	state, serr := b.GitFallbackAuthorState()
	if serr != nil {
		verr := fmt.Errorf("gitid: cannot read fallback author from %s: %w", b.displayPath(b.gitconfigPath), serr)
		if flags.JSON {
			env := newGitFallbackSetEnvelope(flags.DryRun)
			env.Error = verr.Error()
			env.ExitCode = 1
			return finishGitFallbackSet(cmd, true, env, verr)
		}
		return sshFinish(1, verr)
	}
	name, email := resolveFallbackPair(state, flags)

	env := newGitFallbackSetEnvelope(flags.DryRun)
	env.TargetPath = b.displayPath(b.gitconfigPath)
	env.SetName = flags.nameWasSet
	env.SetEmail = flags.emailWasSet
	env.ClearedName = flags.ClearName
	env.ClearedEmail = flags.ClearEmail

	if flags.DryRun {
		res, rerr := cliGitFallbackAuthorApplyInto(b, name, email, lifecyclePolicy{DryRun: true})
		if !flags.JSON {
			plan, _ := b.GitFallbackAuthorPlan(name, email)
			printGitFallbackDryRun(cmd.OutOrStdout(), plan)
		}
		fillGitFallbackSetFromResult(b, &env, res, rerr)
		env.ExitCode = gitWriteExitCode(res, rerr, false, true)
		return finishGitFallbackSet(cmd, flags.JSON, env, rerr)
	}

	policy, perr := confirmationPolicyFrom(cmd, "set git fallback author", stdinTTY, stdoutTTY, flags.Yes, func(preview string) (bool, error) {
		if plan, perr := b.GitFallbackAuthorPlan(name, email); perr == nil {
			printGitFallbackDryRun(cmd.OutOrStdout(), plan)
		}
		fmt.Fprintf(cmd.OutOrStdout(), "%s\nType \"yes\" to confirm: ", preview) //nolint:errcheck
		reader := bufio.NewReader(cmd.InOrStdin())
		line, _ := reader.ReadString('\n')
		return strings.TrimSpace(line) == "yes", nil
	})
	if perr != nil {
		env.Error = perr.Error()
		env.ExitCode = 1
		return finishGitFallbackSet(cmd, flags.JSON, env, perr)
	}

	res, rerr := cliGitFallbackAuthorApplyInto(b, name, email, policy)
	fillGitFallbackSetFromResult(b, &env, res, rerr)
	env.ExitCode = gitWriteExitCode(res, rerr, false, false)
	if !flags.JSON {
		printGitFallbackHuman(cmd, b, res, rerr)
	}
	return finishGitFallbackSet(cmd, flags.JSON, env, rerr)
}

func validateFallbackSetFlags(flags gitFallbackSetFlags) error {
	if flags.nameWasSet && flags.Name == "" {
		return fmt.Errorf("gitid: --name must not be empty (use --clear-name to unset)")
	}
	if flags.emailWasSet && flags.Email == "" {
		return fmt.Errorf("gitid: --email must not be empty (use --clear-email to unset)")
	}
	if flags.nameWasSet && flags.ClearName {
		return fmt.Errorf("gitid: refusing --name together with --clear-name")
	}
	if flags.emailWasSet && flags.ClearEmail {
		return fmt.Errorf("gitid: refusing --email together with --clear-email")
	}
	if !flags.nameWasSet && !flags.emailWasSet && !flags.ClearName && !flags.ClearEmail {
		return fmt.Errorf("gitid: git fallback set requires the flag(s) --name, --email, --clear-name, --clear-email to run headless")
	}
	return nil
}

func resolveFallbackPair(state tuikit.GitFallbackAuthorView, flags gitFallbackSetFlags) (name, email string) {
	name, email = state.Name, state.Email
	if flags.nameWasSet {
		name = flags.Name
	}
	if flags.emailWasSet {
		email = flags.Email
	}
	if flags.ClearName {
		name = ""
	}
	if flags.ClearEmail {
		email = ""
	}
	return name, email
}

type gitOptionRecord struct {
	Key              string `json:"key"`
	Token            string `json:"token"`
	CurrentValue     string `json:"current_value"`
	Provenance       string `json:"provenance"`
	RecommendedValue string `json:"recommended_value"`
	State            string `json:"state"`
	ProbeError       string `json:"probe_error"`
}

func gitOptionRecords(b *realBackend) ([]gitOptionRecord, error) {
	views, err := cliGitOptionStates(b)
	if err != nil {
		return nil, err
	}
	out := make([]gitOptionRecord, 0, len(views))
	for _, v := range views {
		policy, _ := globalgit.PolicyFor(v.Key)
		out = append(out, gitOptionRecord{
			Key:              v.Key,
			Token:            policy.Token,
			CurrentValue:     v.CurrentValue,
			Provenance:       v.Provenance,
			RecommendedValue: v.Recommended,
			State:            gitRowStateName(v),
			ProbeError:       v.ProbeError,
		})
	}
	return out, nil
}

func renderGitOptionsList(w io.Writer, isTTY, jsonOut bool, recs []gitOptionRecord) error {
	if jsonOut {
		if recs == nil {
			recs = []gitOptionRecord{}
		}
		return writeJSON(w, gitOptionsDocument{Schema: gitOptionsSchema, Options: recs})
	}
	if isTTY {
		tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
		fmt.Fprintln(tw, "TOKEN\tKEY\tSTATE\tCURRENT\tRECOMMENDED\tPROVENANCE") //nolint:errcheck
		for _, r := range recs {
			fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\n", r.Token, r.Key, r.State, r.CurrentValue, r.RecommendedValue, r.Provenance) //nolint:errcheck
		}
		return tw.Flush()
	}
	for _, r := range recs {
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\n", r.Token, r.Key, r.State, r.CurrentValue, r.RecommendedValue, r.Provenance) //nolint:errcheck
	}
	return nil
}

func printGitApplyDryRun(w io.Writer, plan tuikit.GlobalGitApplyPlanView) {
	fmt.Fprintln(w, "dry run: would apply global Git options") //nolint:errcheck
	if plan.Diff != "" {
		fmt.Fprintln(w, plan.Diff) //nolint:errcheck
	}
}

func printGitApplyHuman(cmd *cobra.Command, b *realBackend, res lifecycleResult, err error, keys []string) {
	if err != nil {
		if len(res.Restored) > 0 {
			fmt.Fprintf(cmd.ErrOrStderr(), "restored: %s\n", strings.Join(displayMessages(b, res.Restored), "; ")) //nolint:errcheck
		}
		return
	}
	for _, bak := range res.Backups {
		fmt.Fprintf(cmd.OutOrStdout(), "backed up -> %s\n", b.displayPath(bak)) //nolint:errcheck
	}
	for _, a := range res.Advisories {
		fmt.Fprintln(cmd.OutOrStdout(), b.displayMessage(a)) //nolint:errcheck
	}
	fmt.Fprintf(cmd.OutOrStdout(), "applied %s\n", strings.Join(keys, ", ")) //nolint:errcheck
}

func printGitFallbackDryRun(w io.Writer, plan tuikit.GitFallbackAuthorPlanView) {
	fmt.Fprintln(w, "dry run: would apply git fallback author") //nolint:errcheck
	if plan.Diff != "" {
		fmt.Fprintln(w, plan.Diff) //nolint:errcheck
	}
}

func printGitFallbackHuman(cmd *cobra.Command, b *realBackend, res lifecycleResult, err error) {
	if err != nil {
		if len(res.Restored) > 0 {
			fmt.Fprintf(cmd.ErrOrStderr(), "restored: %s\n", strings.Join(displayMessages(b, res.Restored), "; ")) //nolint:errcheck
		}
		return
	}
	for _, bak := range res.Backups {
		fmt.Fprintf(cmd.OutOrStdout(), "backed up -> %s\n", b.displayPath(bak)) //nolint:errcheck
	}
	for _, a := range res.Advisories {
		fmt.Fprintln(cmd.OutOrStdout(), b.displayMessage(a)) //nolint:errcheck
	}
	fmt.Fprintln(cmd.OutOrStdout(), "applied git fallback author") //nolint:errcheck
}

// ---------------------------------------------------------------------------
// JSON envelopes (plan 07-05 Task 2) — the frozen key sets are pinned by
// git_schema_test.go, so an envelope that grows a key silently is a test
// failure, not a contract drift.
// ---------------------------------------------------------------------------

type gitOptionsDocument struct {
	Schema  string            `json:"schema"`
	Options []gitOptionRecord `json:"options"`
}

type gitApplyDocument struct {
	Schema     string   `json:"schema"`
	DryRun     bool     `json:"dry_run"`
	Applied    []string `json:"applied"`
	Declined   []string `json:"declined"`
	TargetPath string   `json:"target_path"`
	Backups    []string `json:"backups"`
	Restored   []string `json:"restored"`
	Advisories []string `json:"advisories"`
	Error      string   `json:"error"`
	ExitCode   int      `json:"exit_code"`
}

type gitFallbackDocument struct {
	Schema      string `json:"schema"`
	Name        string `json:"name"`
	Email       string `json:"email"`
	NameStatus  string `json:"name_status"`
	EmailStatus string `json:"email_status"`
}

type gitFallbackSetDocument struct {
	Schema       string   `json:"schema"`
	DryRun       bool     `json:"dry_run"`
	SetName      bool     `json:"set_name"`
	SetEmail     bool     `json:"set_email"`
	ClearedName  bool     `json:"cleared_name"`
	ClearedEmail bool     `json:"cleared_email"`
	TargetPath   string   `json:"target_path"`
	Backups      []string `json:"backups"`
	Restored     []string `json:"restored"`
	Advisories   []string `json:"advisories"`
	Error        string   `json:"error"`
	ExitCode     int      `json:"exit_code"`
}

func newGitApplyEnvelope(dryRun bool) gitApplyDocument {
	return gitApplyDocument{
		Schema:     gitApplySchema,
		DryRun:     dryRun,
		Applied:    []string{},
		Declined:   []string{},
		Backups:    []string{},
		Restored:   []string{},
		Advisories: []string{},
	}
}

func newGitFallbackSetEnvelope(dryRun bool) gitFallbackSetDocument {
	return gitFallbackSetDocument{
		Schema:     gitFallbackSetSchema,
		DryRun:     dryRun,
		Backups:    []string{},
		Restored:   []string{},
		Advisories: []string{},
	}
}

// finishGitApply writes the apply envelope when requested and returns an error
// whose exit status equals the envelope's own exit-code field on every path —
// success, refusal and rollback alike, so a consumer never has to distinguish
// no output from no advisories.
func finishGitApply(cmd *cobra.Command, jsonOut bool, env gitApplyDocument, err error) error {
	if env.Advisories == nil {
		env.Advisories = []string{}
	}
	if env.Applied == nil {
		env.Applied = []string{}
	}
	if env.Declined == nil {
		env.Declined = []string{}
	}
	if env.Backups == nil {
		env.Backups = []string{}
	}
	if env.Restored == nil {
		env.Restored = []string{}
	}
	if jsonOut {
		if werr := writeJSON(cmd.OutOrStdout(), env); werr != nil {
			return werr
		}
	}
	return sshFinish(env.ExitCode, err)
}

// finishGitFallbackSet mirrors finishGitApply for the fallback-set envelope.
func finishGitFallbackSet(cmd *cobra.Command, jsonOut bool, env gitFallbackSetDocument, err error) error {
	if env.Advisories == nil {
		env.Advisories = []string{}
	}
	if env.Backups == nil {
		env.Backups = []string{}
	}
	if env.Restored == nil {
		env.Restored = []string{}
	}
	if jsonOut {
		if werr := writeJSON(cmd.OutOrStdout(), env); werr != nil {
			return werr
		}
	}
	return sshFinish(env.ExitCode, err)
}

// fillGitApplyFromResult fills the apply envelope from a lifecycleResult.
// tokens are the argv row tokens the command line named (R-5's argv
// vocabulary) — the same spelling a script typed.
func fillGitApplyFromResult(b *realBackend, env *gitApplyDocument, res lifecycleResult, err error, tokens []string) {
	env.Backups = displayPaths(b, res.Backups)
	if env.Backups == nil {
		env.Backups = []string{}
	}
	env.Restored = displayMessages(b, res.Restored)
	if env.Restored == nil {
		env.Restored = []string{}
	}
	env.Advisories = displayMessages(b, res.Advisories)
	if env.Advisories == nil {
		env.Advisories = []string{}
	}
	if err != nil {
		env.Error = err.Error()
		if len(res.Restored) == 0 {
			env.Declined = append([]string{}, tokens...)
			env.Applied = []string{}
		} else {
			env.Applied = []string{}
			env.Declined = []string{}
		}
		return
	}
	env.Applied = append([]string{}, tokens...)
	env.Declined = []string{}
}

// fillGitFallbackSetFromResult fills the fallback-set envelope from a
// lifecycleResult.
func fillGitFallbackSetFromResult(b *realBackend, env *gitFallbackSetDocument, res lifecycleResult, err error) {
	env.Backups = displayPaths(b, res.Backups)
	if env.Backups == nil {
		env.Backups = []string{}
	}
	env.Restored = displayMessages(b, res.Restored)
	if env.Restored == nil {
		env.Restored = []string{}
	}
	env.Advisories = displayMessages(b, res.Advisories)
	if env.Advisories == nil {
		env.Advisories = []string{}
	}
	if err != nil {
		env.Error = err.Error()
	}
}
