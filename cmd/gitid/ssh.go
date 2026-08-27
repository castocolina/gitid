package main

// ssh.go is the real `gitid ssh` command group (plan 06-06): options list /
// options apply / storage show / storage migrate. Both write verbs call the
// SAME UI-free ceremonies the TUI uses — runGlobalSSHApply and
// runSSHStorageMigrate — never a tea.Cmd and never a CLI-only write path.

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"

	tea "charm.land/bubbletea/v2"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"golang.org/x/term"

	"github.com/castocolina/gitid/internal/globalssh"
	"github.com/castocolina/gitid/internal/sshconfig"
	"github.com/castocolina/gitid/internal/tuikit"
)

const (
	sshOptionsSchema = "gitid.ssh.options/v1"
	sshStorageSchema = "gitid.ssh.storage/v1"
	sshApplySchema   = "gitid.ssh.apply/v1"
	sshMigrateSchema = "gitid.ssh.migrate/v1"

	sshLayoutInclude = "include"
	sshLayoutInFile  = "in-file"

	sshApplyRequiredKey = "<key>"
	sshMigrateRequired  = "--to"
)

// cliGlobalSSHApplyInto / cliSSHStorageMigrateInto are the CLI's lifecycle
// seams: thin adapters over the SAME runGlobalSSHApply / runSSHStorageMigrate
// the TUI's commit seams call. Test-only vars so a recording double can
// assert the handler invokes the ceremony exactly once.
var (
	cliGlobalSSHApplyInto = func(b *realBackend, keys []string, p lifecyclePolicy) (lifecycleResult, error) {
		return b.runGlobalSSHApply(keys, p)
	}
	cliSSHStorageMigrateInto = func(b *realBackend, target tuikit.SSHStorageLayout, p lifecyclePolicy) (lifecycleResult, error) {
		return b.runSSHStorageMigrate(target, "", p)
	}
	sshTUILaunch = func(b *realBackend, storageTab bool) error {
		_, err := tea.NewProgram(tuikit.NewAppOnGlobalSSH(b, storageTab)).Run()
		return err
	}
)

// exitCodeError carries a frozen ssh write-verb status so main() can os.Exit
// with the same number the JSON envelope's exit_code field emitted.
type exitCodeError struct {
	code int
	err  error
}

func (e *exitCodeError) Error() string {
	if e.err == nil {
		return fmt.Sprintf("gitid: exit %d", e.code)
	}
	return e.err.Error()
}

func (e *exitCodeError) Unwrap() error { return e.err }

func (e *exitCodeError) ExitCode() int { return e.code }

func exitStatusOf(err error) int {
	if err == nil {
		return 0
	}
	var ee *exitCodeError
	if errors.As(err, &ee) {
		return ee.code
	}
	return 1
}

func sshFinish(code int, err error) error {
	if code == 0 {
		return nil
	}
	if err == nil {
		err = fmt.Errorf("gitid: command failed")
	}
	return &exitCodeError{code: code, err: err}
}

// sshWriteExitCode maps a lifecycleResult plus its error plus the
// --fail-on-advisory opt-in onto the frozen exit-status table.
//
// Zero-on-advisory is the direct consequence of D-04 and D-14: shadowing is
// ADVISORY and never blocking, so a script that pipes gitid into a
// provisioning run must not fail because gitid was honest about a
// pre-existing directive. --fail-on-advisory is the opt-in for a stricter
// script. Never change the default to nonzero without revisiting those
// decisions. A dry run cannot write, so it never returns 3.
func sshWriteExitCode(res lifecycleResult, err error, failOnAdvisory, dryRun bool) int {
	if err != nil {
		if len(res.Restored) > 0 {
			return 2
		}
		return 1
	}
	if dryRun {
		return 0
	}
	if failOnAdvisory && len(res.Advisories) > 0 {
		return 3
	}
	return 0
}

func newSSHCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "ssh",
		Short: "Manage global SSH options and storage layout",
	}
	cmd.AddCommand(newSSHOptionsCmd())
	cmd.AddCommand(newSSHStorageCmd())
	return cmd
}

func newSSHOptionsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "options",
		Short: "List or apply global SSH options",
	}
	cmd.AddCommand(newVerbCmd(newSSHOptionsListVerb()))
	cmd.AddCommand(newVerbCmd(newSSHOptionsApplyVerb()))
	return cmd
}

func newSSHStorageCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "storage",
		Short: "Show or migrate the SSH storage layout",
	}
	cmd.AddCommand(newVerbCmd(newSSHStorageShowVerb()))
	cmd.AddCommand(newVerbCmd(newSSHStorageMigrateVerb()))
	return cmd
}

func newSSHOptionsListVerb() identityVerb {
	var jsonOut bool
	return identityVerb{
		use:   "list",
		short: "List the six global SSH options with current value, state and source",
		args:  cobra.NoArgs,
		bindFlags: func(fs *pflag.FlagSet) {
			fs.BoolVar(&jsonOut, "json", false, "print the frozen gitid.ssh.options/v1 document")
		},
		run: func(cmd *cobra.Command, _ []string) error {
			home, err := resolveHomeForCLI()
			if err != nil {
				return err
			}
			b := newBackendForHome(home)
			recs, err := sshOptionRecords(b)
			if err != nil {
				return err
			}
			isTTY := term.IsTerminal(int(os.Stdout.Fd()))
			return renderSSHOptionsList(cmd.OutOrStdout(), isTTY, jsonOut, recs)
		},
	}
}

type sshApplyFlags struct {
	Yes            bool
	DryRun         bool
	FailOnAdvisory bool
	JSON           bool
}

func newSSHOptionsApplyVerb() identityVerb {
	var flags sshApplyFlags
	return identityVerb{
		use:   "apply [key...]",
		short: "Apply named global SSH options toward their recommended values",
		args:  cobra.ArbitraryArgs,
		bindFlags: func(fs *pflag.FlagSet) {
			fs.BoolVar(&flags.Yes, "yes", false, "skip the confirmation prompt; the timestamped backup is still taken unconditionally")
			fs.BoolVar(&flags.DryRun, "dry-run", false, "print the plan, simulation and shadow report and exit 0 without writing")
			fs.BoolVar(&flags.FailOnAdvisory, "fail-on-advisory", false, "exit 3 when a successful write reports an advisory (default: exit 0)")
			fs.BoolVar(&flags.JSON, "json", false, "print the frozen gitid.ssh.apply/v1 write-result document")
		},
		run: func(cmd *cobra.Command, args []string) error {
			return runSSHOptionsApply(cmd, args, flags, termIsStdinTTY(), termIsStdoutTTY())
		},
	}
}

func newSSHStorageShowVerb() identityVerb {
	var jsonOut bool
	return identityVerb{
		use:   "show",
		short: "Show the current SSH storage layout",
		args:  cobra.NoArgs,
		bindFlags: func(fs *pflag.FlagSet) {
			fs.BoolVar(&jsonOut, "json", false, "print the frozen gitid.ssh.storage/v1 document")
		},
		run: func(cmd *cobra.Command, _ []string) error {
			home, err := resolveHomeForCLI()
			if err != nil {
				return err
			}
			b := newBackendForHome(home)
			doc := buildSSHStorageDocument(b)
			if jsonOut {
				return writeJSON(cmd.OutOrStdout(), doc)
			}
			_, err = fmt.Fprintf(cmd.OutOrStdout(), "layout\t%s\ntarget\t%s\nmain\t%s\ninclude_line\t%v\n",
				doc.Layout, doc.TargetPath, doc.MainConfigPath, doc.IncludeLinePresent)
			return err
		},
	}
}

type sshMigrateFlags struct {
	To     string
	Yes    bool
	DryRun bool
	JSON   bool
}

func newSSHStorageMigrateVerb() identityVerb {
	var flags sshMigrateFlags
	return identityVerb{
		use:   "migrate",
		short: "Migrate SSH storage between include and in-file layouts",
		args:  cobra.NoArgs,
		bindFlags: func(fs *pflag.FlagSet) {
			fs.StringVar(&flags.To, "to", "", "target layout: include or in-file")
			fs.BoolVar(&flags.Yes, "yes", false, "skip the confirmation prompt; the timestamped backup is still taken unconditionally")
			fs.BoolVar(&flags.DryRun, "dry-run", false, "print the migration plan and resulting configuration for both files and exit 0 without writing")
			fs.BoolVar(&flags.JSON, "json", false, "print the frozen gitid.ssh.migrate/v1 write-result document")
		},
		run: func(cmd *cobra.Command, _ []string) error {
			return runSSHStorageMigrateVerb(cmd, flags, termIsStdinTTY(), termIsStdoutTTY())
		},
	}
}

func runSSHOptionsApply(cmd *cobra.Command, keys []string, flags sshApplyFlags, stdinTTY, stdoutTTY bool) error {
	supplied := map[string]bool{sshApplyRequiredKey: len(keys) > 0}
	outcome, missing := depthResolver{
		required:  []string{sshApplyRequiredKey},
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
		return sshTUILaunch(newBackendForHome(home), false)
	case resolveMissingFlags:
		err := missingFlagErr("ssh options apply", missing)
		if flags.JSON {
			env := newApplyEnvelope(flags.DryRun)
			env.Error = err.Error()
			env.ExitCode = 1
			return finishApply(cmd, true, env, err)
		}
		return sshFinish(1, err)
	case resolveHeadless:
	}

	home, err := resolveHomeForCLI()
	if err != nil {
		return sshFinish(1, err)
	}
	b := newBackendForHome(home)

	env := newApplyEnvelope(flags.DryRun)
	env.TargetPath = b.displayPath(b.globalsTargetPath())

	if flags.DryRun {
		res, rerr := cliGlobalSSHApplyInto(b, keys, lifecyclePolicy{DryRun: true})
		fillApplyFromResult(b, &env, res, rerr, keys)
		if !flags.JSON {
			plan, _ := b.GlobalSSHApplyPlan(keys)
			printApplyDryRun(cmd.OutOrStdout(), plan)
		}
		env.ExitCode = sshWriteExitCode(res, rerr, flags.FailOnAdvisory, true)
		return finishApply(cmd, flags.JSON, env, rerr)
	}

	policy, perr := confirmationPolicyFrom(cmd, "apply global SSH options", stdinTTY, stdoutTTY, flags.Yes, func(preview string) (bool, error) {
		// WR-08: show the resolved target/diff/shadow-warning preview before
		// asking — the interactive CLI user must see the SAME ceremony
		// information the TUI and --dry-run show, not a bare option-name
		// question.
		if plan, perr := b.GlobalSSHApplyPlan(keys); perr == nil {
			printApplyDryRun(cmd.OutOrStdout(), plan)
		}
		fmt.Fprintf(cmd.OutOrStdout(), "%s\nType \"yes\" to confirm: ", preview) //nolint:errcheck
		reader := bufio.NewReader(cmd.InOrStdin())
		line, _ := reader.ReadString('\n')
		return strings.TrimSpace(line) == "yes", nil
	})
	if perr != nil {
		env.Declined = append([]string{}, keys...)
		env.Applied = []string{}
		env.Error = perr.Error()
		env.ExitCode = 1
		return finishApply(cmd, flags.JSON, env, perr)
	}

	res, rerr := cliGlobalSSHApplyInto(b, keys, policy)
	fillApplyFromResult(b, &env, res, rerr, keys)
	env.ExitCode = sshWriteExitCode(res, rerr, flags.FailOnAdvisory, false)
	if !flags.JSON {
		printApplyHuman(cmd, b, res, rerr, keys)
	}
	return finishApply(cmd, flags.JSON, env, rerr)
}

func runSSHStorageMigrateVerb(cmd *cobra.Command, flags sshMigrateFlags, stdinTTY, stdoutTTY bool) error {
	supplied := map[string]bool{sshMigrateRequired: strings.TrimSpace(flags.To) != ""}
	outcome, missing := depthResolver{
		required:  []string{sshMigrateRequired},
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
		return sshTUILaunch(newBackendForHome(home), true)
	case resolveMissingFlags:
		err := missingFlagErr("ssh storage migrate", missing)
		if flags.JSON {
			env := newMigrateEnvelope(flags.DryRun)
			env.Error = err.Error()
			env.ExitCode = 1
			return finishMigrate(cmd, true, env, err)
		}
		return sshFinish(1, err)
	case resolveHeadless:
	}

	to, ok := wireToStorage(flags.To)
	if !ok {
		err := fmt.Errorf("gitid: unknown storage layout %q (want include or in-file)", flags.To)
		env := newMigrateEnvelope(false)
		env.ToLayout = strings.TrimSpace(flags.To)
		env.Error = err.Error()
		env.ExitCode = 1
		return finishMigrate(cmd, flags.JSON, env, err)
	}

	home, err := resolveHomeForCLI()
	if err != nil {
		return sshFinish(1, err)
	}
	b := newBackendForHome(home)
	st := b.storage()
	fromWire := sshLayoutWire(st)
	toWire := storageToWire(to)
	env := newMigrateEnvelope(flags.DryRun)
	env.FromLayout = fromWire
	env.ToLayout = toWire

	if fromWire == toWire {
		err := fmt.Errorf("gitid: storage layout is already %s — nothing to migrate", toWire)
		env.Error = err.Error()
		env.ExitCode = 1
		return finishMigrate(cmd, flags.JSON, env, err)
	}

	direction := sshconfig.MigrateToInclude
	if to == tuikit.StorageSentinel {
		direction = sshconfig.MigrateToInFile
	}
	plan, planErr := sshconfig.PlanMigration(direction, newMigrateDeps(b.sshConfigPath, filepath.Join(b.includeDir, gitidConfigFileName), b.managedAliases()))
	if planErr == nil {
		env.MovedIdentities = append([]string{}, plan.MovedIdentities...)
		env.MovedGlobals = len(plan.MovedGlobals) > 0
		if flags.DryRun && !flags.JSON {
			printMigrateDryRun(cmd.OutOrStdout(), b, plan)
		}
	}

	if flags.DryRun {
		res, rerr := cliSSHStorageMigrateInto(b, to, lifecyclePolicy{DryRun: true})
		if rerr != nil && planErr != nil {
			rerr = planErr
		}
		fillMigrateFromResult(b, &env, res, rerr)
		if planErr != nil && rerr == nil {
			env.Error = planErr.Error()
			rerr = planErr
		}
		env.ExitCode = sshWriteExitCode(res, rerr, false, true)
		return finishMigrate(cmd, flags.JSON, env, rerr)
	}

	policy, perr := confirmationPolicyFrom(cmd, "migrate SSH storage", stdinTTY, stdoutTTY, flags.Yes, func(preview string) (bool, error) {
		// WR-08: show the resolved plan diff before asking — the interactive
		// CLI user must see the SAME ceremony information the TUI and
		// --dry-run show, not a bare layout-name question.
		if planErr == nil {
			printMigrateDryRun(cmd.OutOrStdout(), b, plan)
		}
		fmt.Fprintf(cmd.OutOrStdout(), "%s\nType \"yes\" to confirm: ", preview) //nolint:errcheck
		reader := bufio.NewReader(cmd.InOrStdin())
		line, _ := reader.ReadString('\n')
		return strings.TrimSpace(line) == "yes", nil
	})
	if perr != nil {
		env.Error = perr.Error()
		env.ExitCode = 1
		return finishMigrate(cmd, flags.JSON, env, perr)
	}

	res, rerr := cliSSHStorageMigrateInto(b, to, policy)
	fillMigrateFromResult(b, &env, res, rerr)
	env.ExitCode = sshWriteExitCode(res, rerr, false, false)
	if !flags.JSON {
		printMigrateHuman(cmd, b, res, rerr, toWire)
	}
	return finishMigrate(cmd, flags.JSON, env, rerr)
}

func finishApply(cmd *cobra.Command, jsonOut bool, env sshApplyDocument, err error) error {
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

func finishMigrate(cmd *cobra.Command, jsonOut bool, env sshMigrateDocument, err error) error {
	if env.MovedIdentities == nil {
		env.MovedIdentities = []string{}
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

// fillApplyFromResult fills env from res/err/keys. WR-06: the success path is
// identical whether the caller is a dry run or a real apply — dryRun was a
// dead parameter (the two branches it selected between were byte-identical)
// and has been removed.
func fillApplyFromResult(b *realBackend, env *sshApplyDocument, res lifecycleResult, err error, keys []string) {
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
	env.SimulationInconclusive = res.SimulationInconclusive
	if res.SimulationInconclusive {
		env.SimulationNote = globalSSHSimInconclusiveNote
	}
	if err != nil {
		env.Error = err.Error()
		if len(res.Restored) == 0 {
			env.Declined = append([]string{}, keys...)
			env.Applied = []string{}
		} else {
			env.Applied = []string{}
			env.Declined = []string{}
		}
		return
	}
	env.Applied = append([]string{}, keys...)
	env.Declined = []string{}
}

func fillMigrateFromResult(b *realBackend, env *sshMigrateDocument, res lifecycleResult, err error) {
	env.Backups = displayPaths(b, res.Backups)
	if env.Backups == nil {
		env.Backups = []string{}
	}
	env.Restored = displayMessages(b, res.Restored)
	if env.Restored == nil {
		env.Restored = []string{}
	}
	if err != nil {
		env.Error = err.Error()
	}
}

// printApplyDryRun prints the dry-run diff/warnings. WR-06: jsonOut was a
// dead parameter — the single call site already nests this call inside
// `if !flags.JSON`, so the guard here could never fire on the JSON path.
func printApplyDryRun(w io.Writer, plan tuikit.GlobalSSHApplyPlanView) {
	fmt.Fprintln(w, "dry run: would apply global SSH options") //nolint:errcheck
	if plan.Diff != "" {
		fmt.Fprintln(w, plan.Diff) //nolint:errcheck
	}
	for _, warn := range plan.ShadowWarnings {
		fmt.Fprintln(w, warn) //nolint:errcheck
	}
	if plan.SimulationInconclusive && plan.SimulationNote != "" {
		fmt.Fprintln(w, plan.SimulationNote) //nolint:errcheck
	}
}

func printApplyHuman(cmd *cobra.Command, b *realBackend, res lifecycleResult, err error, keys []string) {
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

func printMigrateDryRun(w io.Writer, b *realBackend, plan sshconfig.MigrationPlan) {
	fmt.Fprintln(w, "dry run: would migrate SSH storage") //nolint:errcheck
	if plan.Diff != "" {
		fmt.Fprint(w, plan.Diff) //nolint:errcheck
	}
	fmt.Fprintf(w, "source after (%s):\n%s\n", b.displayPath(plan.SourcePath), plan.SourceAfter) //nolint:errcheck
	fmt.Fprintf(w, "dest after (%s):\n%s\n", b.displayPath(plan.DestPath), plan.DestAfter)       //nolint:errcheck
}

func printMigrateHuman(cmd *cobra.Command, b *realBackend, res lifecycleResult, err error, to string) {
	if err != nil {
		if len(res.Restored) > 0 {
			fmt.Fprintf(cmd.ErrOrStderr(), "restored: %s\n", strings.Join(displayMessages(b, res.Restored), "; ")) //nolint:errcheck
		}
		return
	}
	for _, bak := range res.Backups {
		fmt.Fprintf(cmd.OutOrStdout(), "backed up -> %s\n", b.displayPath(bak)) //nolint:errcheck
	}
	fmt.Fprintf(cmd.OutOrStdout(), "migrated SSH storage to %s\n", to) //nolint:errcheck
}

// ---------------------------------------------------------------------------
// JSON envelopes
// ---------------------------------------------------------------------------

type sshOptionRecord struct {
	Key                 string `json:"key"`
	CurrentValue        string `json:"current_value"`
	RecommendedValue    string `json:"recommended_value"`
	Risk                string `json:"risk"`
	Scope               string `json:"scope"`
	State               string `json:"state"`
	Source              string `json:"source"`
	SourceFile          string `json:"source_file"`
	SourceLine          int    `json:"source_line"`
	NotApplicableReason string `json:"not_applicable_reason"`
	VersionNote         string `json:"version_note"`
	ProbeError          string `json:"probe_error"`
}

type sshOptionsDocument struct {
	Schema  string            `json:"schema"`
	Options []sshOptionRecord `json:"options"`
}

type sshStorageDocument struct {
	Schema             string `json:"schema"`
	Layout             string `json:"layout"`
	TargetPath         string `json:"target_path"`
	MainConfigPath     string `json:"main_config_path"`
	IncludeLinePresent bool   `json:"include_line_present"`
}

type sshApplyDocument struct {
	Schema                 string   `json:"schema"`
	DryRun                 bool     `json:"dry_run"`
	Applied                []string `json:"applied"`
	Declined               []string `json:"declined"`
	TargetPath             string   `json:"target_path"`
	Backups                []string `json:"backups"`
	Restored               []string `json:"restored"`
	Advisories             []string `json:"advisories"`
	SimulationInconclusive bool     `json:"simulation_inconclusive"`
	SimulationNote         string   `json:"simulation_note"`
	Error                  string   `json:"error"`
	ExitCode               int      `json:"exit_code"`
}

type sshMigrateDocument struct {
	Schema          string   `json:"schema"`
	DryRun          bool     `json:"dry_run"`
	FromLayout      string   `json:"from_layout"`
	ToLayout        string   `json:"to_layout"`
	MovedIdentities []string `json:"moved_identities"`
	MovedGlobals    bool     `json:"moved_globals"`
	Backups         []string `json:"backups"`
	Restored        []string `json:"restored"`
	Error           string   `json:"error"`
	ExitCode        int      `json:"exit_code"`
}

func newApplyEnvelope(dryRun bool) sshApplyDocument {
	return sshApplyDocument{
		Schema:     sshApplySchema,
		DryRun:     dryRun,
		Applied:    []string{},
		Declined:   []string{},
		Backups:    []string{},
		Restored:   []string{},
		Advisories: []string{},
	}
}

func newMigrateEnvelope(dryRun bool) sshMigrateDocument {
	return sshMigrateDocument{
		Schema:          sshMigrateSchema,
		DryRun:          dryRun,
		MovedIdentities: []string{},
		Backups:         []string{},
		Restored:        []string{},
	}
}

func sshOptionRecords(b *realBackend) ([]sshOptionRecord, error) {
	if b.initErr != nil {
		return nil, b.initErr
	}
	statuses := globalssh.Statuses(globalssh.BuildProbeDeps(b.sshConfigPath))
	sshVersion := b.readSSHVersion()
	out := make([]sshOptionRecord, 0, len(statuses))
	for _, st := range statuses {
		policy, _ := globalssh.PolicyFor(st.Key)
		rec := sshOptionRecord{
			Key:                 st.Key,
			CurrentValue:        st.CurrentValue,
			RecommendedValue:    st.RecommendedValue,
			Risk:                strings.ToLower(st.Risk),
			Scope:               policy.Scope,
			State:               sshStateWire(st.State),
			Source:              sshSourceWire(st.Source),
			SourceFile:          b.displayPath(st.SourceFile),
			SourceLine:          st.SourceLine,
			NotApplicableReason: sshNAReasonWire(st.NotApplicableReason),
			ProbeError:          st.ProbeError,
		}
		if policy.MinOpenSSH != "" {
			outcome, note := globalssh.VersionGate(sshVersion, policy)
			rec.VersionNote = note
			switch outcome {
			case globalssh.VersionTooOld:
				rec.State = "not-applicable"
				rec.NotApplicableReason = "version-too-old"
			case globalssh.VersionUnverified:
				rec.State = "not-applicable"
				rec.NotApplicableReason = "version-unverified"
			}
		}
		out = append(out, rec)
	}
	return out, nil
}

func buildSSHStorageDocument(b *realBackend) sshStorageDocument {
	st := b.storage()
	return sshStorageDocument{
		Schema:             sshStorageSchema,
		Layout:             sshLayoutWire(st),
		TargetPath:         b.displayPath(st.targetPath),
		MainConfigPath:     b.displayPath(b.sshConfigPath),
		IncludeLinePresent: b.hasIncludeLine(),
	}
}

func renderSSHOptionsList(w io.Writer, isTTY, jsonOut bool, recs []sshOptionRecord) error {
	if jsonOut {
		if recs == nil {
			recs = []sshOptionRecord{}
		}
		return writeJSON(w, sshOptionsDocument{Schema: sshOptionsSchema, Options: recs})
	}
	if isTTY {
		tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
		fmt.Fprintln(tw, "KEY\tSTATE\tCURRENT\tRECOMMENDED\tRISK\tSOURCE") //nolint:errcheck
		for _, r := range recs {
			fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\n", r.Key, r.State, r.CurrentValue, r.RecommendedValue, r.Risk, r.Source) //nolint:errcheck
		}
		return tw.Flush()
	}
	for _, r := range recs {
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\n", r.Key, r.State, r.CurrentValue, r.RecommendedValue, r.Risk, r.Source) //nolint:errcheck
	}
	return nil
}

func sshStateWire(s globalssh.OptionState) string {
	switch s {
	case globalssh.StateAlreadySet:
		return "already-set"
	case globalssh.StateDiffers:
		return "differs"
	case globalssh.StateNotApplicable:
		return "not-applicable"
	default:
		return "needs-action"
	}
}

func sshSourceWire(s globalssh.SourceClass) string {
	switch s {
	case globalssh.SourceGitidParsed:
		return "gitid-parsed"
	case globalssh.SourceOutsideGitid:
		return "outside-gitid"
	case globalssh.SourceSystemFile:
		return "system-file"
	case globalssh.SourceBaseline:
		return "baseline"
	default:
		return "inconclusive"
	}
}

func sshNAReasonWire(r globalssh.NotApplicableReason) string {
	switch r {
	case globalssh.ReasonPlatform:
		return "platform"
	case globalssh.ReasonVersionTooOld:
		return "version-too-old"
	case globalssh.ReasonVersionUnverified:
		return "version-unverified"
	case globalssh.ReasonNothingToVerify:
		return "nothing-to-verify"
	case globalssh.ReasonProbeFailed:
		return "probe-failed"
	default:
		return "none"
	}
}

func sshLayoutWire(st storageLayout) string {
	if st.includeLayout {
		return sshLayoutInclude
	}
	return sshLayoutInFile
}

func storageToWire(layout tuikit.SSHStorageLayout) string {
	if layout == tuikit.StorageInclude {
		return sshLayoutInclude
	}
	return sshLayoutInFile
}

func wireToStorage(s string) (tuikit.SSHStorageLayout, bool) {
	switch strings.TrimSpace(s) {
	case sshLayoutInclude:
		return tuikit.StorageInclude, true
	case sshLayoutInFile:
		return tuikit.StorageSentinel, true
	default:
		return "", false
	}
}
