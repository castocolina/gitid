package main

// identity_create.go implements `gitid identity create` — the D-02 adaptive-
// depth create verb. A complete flag set runs headless; an incomplete set on
// a real terminal opens the app with the create wizard pre-filled from the
// flags that were supplied; an incomplete set without a terminal exits
// non-zero naming exactly the missing flags.
//
// D-02's one-chokepoint claim holds for create exactly as for the key verbs:
// the write path is the SAME commitCreateTransaction the TUI's CommitCreate
// seam calls, never a second write pipeline. The two connectivity gate stages
// are the SAME backend methods (TestStage1/TestStage2) the TUI wizard's test
// steps invoke, and the store gate (storeUnlockedFor) is the same one
// CommitCreate consults. A recording double test overrides cliPreWriteGate
// and commitCreateInto to prove the handler itself performs no stage and
// produces no file effect (review R-11-CLI).

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"golang.org/x/term"

	"github.com/castocolina/gitid/internal/gitconfig"
	"github.com/castocolina/gitid/internal/identity"
	"github.com/castocolina/gitid/internal/keygen"
	"github.com/castocolina/gitid/internal/platform"
	"github.com/castocolina/gitid/internal/sshconfig"
	"github.com/castocolina/gitid/internal/tester"
	"github.com/castocolina/gitid/internal/tuikit"
)

// createRequiredFlags are the flags an `identity create` invocation must
// supply to run headless (D-02). Any one missing routes the invocation to the
// pre-filled wizard (on a terminal) or the missing-flags error (off one).
var createRequiredFlags = []string{"name", "provider", "git-name", "git-email"}

// identityCreateFlags carries the create verb's flags as a plain struct so
// runIdentityCreate stays independently testable without a *cobra.Command.
type identityCreateFlags struct {
	Name      string
	Provider  string
	SSHHost   string
	Hostname  string
	Port      string
	Algorithm string
	ReuseKey  string
	GitName   string
	GitEmail  string
	Strategy  string
	GitDir    string
	ForceSSH  bool
	Yes       bool
	DryRun    bool
	NoUpload  bool
}

// newIdentityCreateVerb builds the `create` verb spec.
func newIdentityCreateVerb() identityVerb {
	var flags identityCreateFlags
	return identityVerb{
		use:     "create",
		aliases: nil,
		short:   "Create a new gitid-managed identity",
		args:    cobra.NoArgs,
		bindFlags: func(fs *pflag.FlagSet) {
			fs.StringVar(&flags.Name, "name", "", "identity name (becomes the SSH Host alias prefix and the key filename)")
			fs.StringVar(&flags.Provider, "provider", "", "provider host (github.com/gitlab.com/bitbucket.org or a custom host)")
			fs.StringVar(&flags.SSHHost, "ssh-host", "", "full SSH Host alias (default <name>.<provider>)")
			fs.StringVar(&flags.Hostname, "hostname", "", "real SSH endpoint (default: the provider's recipe alt-SSH hostname)")
			fs.StringVar(&flags.Port, "port", "", "SSH port (default: the provider's recipe port, 443)")
			fs.StringVar(&flags.Algorithm, "algorithm", "ed25519", "key algorithm (ed25519, rsa-4096); ignored when --reuse-key is given")
			fs.StringVar(&flags.ReuseKey, "reuse-key", "", "path to an existing key to reuse instead of generating one")
			fs.StringVar(&flags.GitName, "git-name", "", "Git author name (user.name)")
			fs.StringVar(&flags.GitEmail, "git-email", "", "Git author email (user.email; byte-identical to the allowed_signers principal)")
			fs.StringVar(&flags.Strategy, "strategy", "gitdir", "includeIf match strategy: gitdir, hasconfig, or both")
			fs.StringVar(&flags.GitDir, "git-dir", "", "gitdir includeIf match path (default ~/git/<name>/)")
			// WR-06: default false — the TUI exposes this as a user-visible
			// toggle, and Reconstruct deliberately never DEFAULTS this value
			// when reading (CR-09); writing it unconditionally on the CLI
			// was an unrequested MACHINE-GLOBAL rewrite of every HTTPS clone
			// URL for this provider, for every repository, including ones
			// unrelated to gitid.
			fs.BoolVar(&flags.ForceSSH, "force-ssh", false, "write the machine-global insteadOf rewrite so HTTPS clone URLs for this provider resolve over SSH (default: off — same toggle the TUI exposes)")
			fs.BoolVar(&flags.Yes, "yes", false, "skip the confirmation prompt; the timestamped backup is still taken unconditionally")
			fs.BoolVar(&flags.DryRun, "dry-run", false, dryRunCreateCloneFlagHelp)
			fs.BoolVar(&flags.NoUpload, "no-upload", false, noUploadFlagHelp)
		},
		run: func(cmd *cobra.Command, _ []string) error {
			return runIdentityCreate(cmd, flags, isTTY(os.Stdin.Fd()), isTTY(os.Stdout.Fd()))
		},
	}
}

// isTTY wraps golang.org/x/term, kept as a tiny helper so every verb's run
// closure reads the two terminal facts the same way.
func isTTY(fd uintptr) bool { return term.IsTerminal(int(fd)) }

// runIdentityCreate is the D-02 adaptive-depth entry point: decide headless
// vs pre-filled TUI vs missing-flags error, then (headless) run the full
// create ceremony.
func runIdentityCreate(cmd *cobra.Command, flags identityCreateFlags, stdinTTY, stdoutTTY bool) error {
	res, missing := depthResolver{
		required: createRequiredFlags,
		supplied: map[string]bool{
			"name":      flags.Name != "",
			"provider":  flags.Provider != "",
			"git-name":  flags.GitName != "",
			"git-email": flags.GitEmail != "",
		},
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
		pre := createPrefillFromFlags(flags)
		return runPrefilledTUI(b, &pre)
	case resolveMissingFlags:
		return missingFlagErr("identity create", missing)
	case resolveHeadless:
	}

	home, err := resolveHomeForCLI()
	if err != nil {
		return err
	}
	b := newBackendForHome(home)
	in, id, err := createInputFromCreateFlags(b, flags)
	if err != nil {
		return err
	}
	return runCreateCeremony(cmd, b, in, id, flags.Yes, flags.DryRun, flags.NoUpload, stdinTTY, stdoutTTY)
}

// createInputFromCreateFlags derives the CreateInput + DemoIdentity a create
// writes from the flag set, validating the alias/host block with the SAME
// validator the confirmed write uses (T-05-22).
func createInputFromCreateFlags(b *realBackend, flags identityCreateFlags) (identity.CreateInput, tuikit.DemoIdentity, error) {
	name := strings.TrimSpace(flags.Name)
	provider := strings.TrimSpace(flags.Provider)
	hostname := strings.TrimSpace(flags.Hostname)

	// CR-04: validate the identity name, git email, and provider BEFORE
	// anything is derived from them. name in particular flows unvalidated
	// into FragmentPath (filepath.Join(b.fragmentDir, name)) and, via
	// createInput/keygen.KeyPaths, into the ~/.ssh/id_<algo>_<name> key
	// paths — validateToken (below, via ValidateHostBlock) rejects
	// whitespace and shell metacharacters but NOT '/' or '..', so an
	// unvalidated name such as "../.bashrc" would resolve those writes to
	// an arbitrary in-home path. ValidateName's charset
	// (^[A-Za-z0-9._-]+$) rejects '/' outright.
	if err := identity.ValidateName(name); err != nil {
		return identity.CreateInput{}, tuikit.DemoIdentity{}, fmt.Errorf("gitid: identity create: %w", err)
	}
	if err := identity.ValidateEmail(strings.TrimSpace(flags.GitEmail)); err != nil {
		return identity.CreateInput{}, tuikit.DemoIdentity{}, fmt.Errorf("gitid: identity create: %w", err)
	}
	if err := identity.ValidateProvider(provider); err != nil {
		return identity.CreateInput{}, tuikit.DemoIdentity{}, fmt.Errorf("gitid: identity create: %w", err)
	}

	port := strings.TrimSpace(flags.Port)
	if hostname == "" {
		hostname = identity.DefaultHostname(provider)
	}
	portNum := atoiOr(port, 0)
	if portNum == 0 {
		portNum = identity.DefaultPort()
	}
	alias := strings.TrimSpace(flags.SSHHost)
	if alias == "" {
		alias = identity.DefaultAlias(name, provider)
	}
	reuseKey := strings.TrimSpace(flags.ReuseKey)
	keyForValidation := "~/.ssh/id_" + sanitizeAlgo(strings.TrimSpace(flags.Algorithm)) + "_" + name
	if reuseKey != "" {
		keyForValidation = reuseKey
	}
	if verr := sshconfig.ValidateHostBlock(alias, hostname, strconv.Itoa(portNum), keyForValidation); verr != nil {
		return identity.CreateInput{}, tuikit.DemoIdentity{}, fmt.Errorf("gitid: identity create: %w", verr)
	}

	strategy := strings.TrimSpace(flags.Strategy)
	switch strategy {
	case "", "gitdir", "hasconfig", "both":
	default:
		return identity.CreateInput{}, tuikit.DemoIdentity{}, fmt.Errorf("gitid: identity create: invalid --strategy %q (want gitdir, hasconfig, or both)", strategy)
	}
	if strategy == "" {
		strategy = "gitdir"
	}
	gitDir := strings.TrimSpace(flags.GitDir)
	if gitDir == "" {
		gitDir = "~/git/" + name + "/"
	}

	algo := strings.TrimSpace(flags.Algorithm)
	if algo == "" {
		algo = "ed25519"
	}

	in := identity.CreateInput{
		Name:               name,
		GitName:            strings.TrimSpace(flags.GitName),
		GitEmail:           strings.TrimSpace(flags.GitEmail),
		Provider:           provider,
		Algo:               algo,
		Alias:              alias,
		Hostname:           hostname,
		Port:               portNum,
		ReuseKeyPath:       reuseKey,
		FragmentPath:       filepath.Join(b.fragmentDir, name),
		GitconfigPath:      b.gitconfigPath,
		SSHConfigPath:      b.storageTargetPath(),
		AllowedSignersPath: b.allowedSigners,
		GlobalsGOOS:        platform.CurrentOS(),
	}
	id := tuikit.DemoIdentity{
		Name:            name,
		SSHHost:         alias,
		Hostname:        hostname,
		Port:            portNum,
		Provider:        provider,
		GitConfigured:   true,
		GitName:         in.GitName,
		GitEmail:        in.GitEmail,
		MatchStrategy:   strategy,
		GitDir:          gitDir,
		PublicKeyPath:   keyForValidation + ".pub",
		ForceSSH:        flags.ForceSSH,
		Algorithm:       strings.TrimSpace(flags.Algorithm),
		ReuseKeyPath:    reuseKey,
		GitFragmentPath: in.FragmentPath,
	}
	return in, id, nil
}

// sanitizeAlgo strips anything that is not [a-zA-Z0-9_-] from an algorithm id
// so a malicious flag value can never reach a key filename.
func sanitizeAlgo(algo string) string {
	var b strings.Builder
	for _, r := range algo {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '_', r == '-':
			b.WriteRune(r)
		}
	}
	return b.String()
}

// createPrefillFromFlags builds the wizard pre-fill value (a ClonePrefillView)
// for the D-02 pre-filled-TUI branch from the flags that were supplied.
func createPrefillFromFlags(flags identityCreateFlags) tuikit.ClonePrefillView {
	pre := tuikit.ClonePrefillView{
		CloneName:     strings.TrimSpace(flags.Name),
		Hostname:      strings.TrimSpace(flags.Hostname),
		Port:          strings.TrimSpace(flags.Port),
		GitName:       strings.TrimSpace(flags.GitName),
		GitEmail:      strings.TrimSpace(flags.GitEmail),
		MatchStrategy: strings.TrimSpace(flags.Strategy),
		GitDir:        strings.TrimSpace(flags.GitDir),
		ReuseKeyPath:  strings.TrimSpace(flags.ReuseKey),
	}
	provider := strings.TrimSpace(flags.Provider)
	if pre.Hostname == "" {
		pre.Hostname = identity.DefaultHostname(provider)
	}
	if pre.Port == "" {
		pre.Port = strconv.Itoa(identity.DefaultPort())
	}
	pre.AliasPrefix = strings.TrimSpace(flags.Name)
	if pre.AliasPrefix == "" {
		pre.AliasPrefix = provider
	}
	return pre
}

// runPrefilledTUI launches the approved app shell with the create wizard
// already open and pre-filled (D-02's pre-filled branch). It shares the exact
// tea.NewProgram launch runApp uses.
func runPrefilledTUI(b *realBackend, pre *tuikit.ClonePrefillView) error {
	if pre == nil {
		_, err := tea.NewProgram(tuikit.NewApp(b)).Run()
		return err
	}
	_, err := tea.NewProgram(tuikit.NewAppPrefilled(b, pre)).Run()
	return err
}

// ---------------------------------------------------------------------------
// The create ceremony (shared by create and clone)
// ---------------------------------------------------------------------------

// cliPreWriteGate runs the create ceremony's two connectivity stages against
// the staged key through the SAME backend methods the TUI wizard's test steps
// call (TestStage1/TestStage2). It is a test-only var seam so a recording
// double can prove the handler itself contributes no stage; the default is
// the thin adapter over the shared methods. The STORE gate (storeUnlockedFor)
// is consulted by the real-run caller, not here, so a dry run can print hard
// failure outcomes instead of erroring out (the per-verb dry-run contract).
var cliPreWriteGate = func(b *realBackend, in identity.CreateInput, id tuikit.DemoIdentity) (tuikit.WizardStageMsg, tuikit.WizardStageMsg, error) {
	spec := tuikit.CreateSpec{
		Identity:     id.Name,
		Alias:        in.Alias,
		Hostname:     in.Hostname,
		Port:         strconv.Itoa(in.Port),
		Provider:     in.Provider,
		ReuseKeyPath: in.ReuseKeyPath,
		Algorithm:    id.Algorithm,
	}
	msg1, ok := b.TestStage1(spec)().(tuikit.WizardStageMsg)
	if !ok {
		return tuikit.WizardStageMsg{}, tuikit.WizardStageMsg{}, fmt.Errorf("gitid: internal: create stage 1 did not deliver a WizardStageMsg")
	}
	msg2, ok := b.TestStage2(spec)().(tuikit.WizardStageMsg)
	if !ok {
		return msg1, tuikit.WizardStageMsg{}, fmt.Errorf("gitid: internal: create stage 2 did not deliver a WizardStageMsg")
	}
	return msg1, msg2, nil
}

// commitCreateInto is the create lifecycle seam: the ONE create write
// function the CLI handler calls, defaulting to the SAME commitCreateTransaction
// the TUI's CommitCreate seam reaches (review R-11-CLI). test-only var so a
// recording double can assert the handler invokes the lifecycle exactly once.
var commitCreateInto = func(b *realBackend, in identity.CreateInput, staged identity.StagedKey, id tuikit.DemoIdentity) ([]string, error) {
	return b.commitCreateTransaction(in, staged, id)
}

// runCreateCeremony is the headless create/clone ceremony: confirmation gate,
// the two connectivity stages, the store gate, the lifecycle write, and the
// backup receipt. A dry run runs the stages, prints outcomes + previews,
// cleans up the staged key, and stops.
func runCreateCeremony(cmd *cobra.Command, b *realBackend, in identity.CreateInput, id tuikit.DemoIdentity, yes, dryRun, noUpload bool, stdinTTY, stdoutTTY bool) error {
	if dryRun {
		return runCreateDryRun(cmd, b, in, id, noUpload)
	}

	policy, err := confirmationPolicyFrom(cmd, "create "+in.Name, stdinTTY, stdoutTTY, yes, func(string) (bool, error) {
		fmt.Fprintf(cmd.OutOrStdout(), "Create identity %q (alias %s) and write its managed artifacts? Type \"yes\" to confirm: ", in.Name, in.Alias) //nolint:errcheck // best-effort prompt
		return confirmYes(cmd)
	})
	if err != nil {
		return err
	}

	stage1, stage2, err := cliPreWriteGate(b, in, id)
	if err != nil {
		return err
	}
	if stage1.Result.Outcome == tuikit.TestOutcomeFailure || stage2.Result.Outcome == tuikit.TestOutcomeFailure {
		return fmt.Errorf("gitid: create %q aborted: stage 1 = %s, stage 2 = %s", in.Name, testOutcomeLabel(stage1.Result.Outcome), testOutcomeLabel(stage2.Result.Outcome))
	}
	if !b.storeUnlockedFor(in) {
		return fmt.Errorf("gitid: refusing to write %q: the connectivity test failed or is stale, so nothing was proven about the provider", in.Name)
	}

	// The confirmation gate (review R2-03): --yes suppressed the prompt at
	// policy construction; an interactive invocation answers it here; a
	// non-interactive run without --yes was already refused above.
	preview := createPreviewLine(in, stage1, stage2)
	authorized, cerr := b.confirmGate(policy, preview)
	if cerr != nil {
		return cerr
	}
	if !authorized {
		return fmt.Errorf("gitid: cancelled create of %q", in.Name)
	}

	staged, err := b.stagedKeyFor(in, in.ReuseKeyPath)
	if err != nil {
		return err
	}
	backups, err := commitCreateInto(b, in, staged, id)
	if err != nil {
		return err
	}
	for _, bak := range backups {
		fmt.Fprintf(cmd.OutOrStdout(), "backed up -> %s\n", b.displayPath(bak)) //nolint:errcheck // best-effort stdout
	}
	fmt.Fprintf(cmd.OutOrStdout(), "created %q (alias %s)\n", in.Name, in.Alias) //nolint:errcheck // best-effort stdout

	// D-03/D-11: the upload step never alters this function's control flow
	// or its returned error — the exit code is decided solely by the
	// PRIMARY operation (create's own write, already committed above).
	// staged.FinalPubPath now exists on disk: commitCreateInto just wrote
	// it, so runUploadFor can read it directly with no further staging.
	runUploadStep(cmd.OutOrStdout(), b, uploadRequest{Identity: in.Name, Hostname: in.Hostname, PubPath: staged.FinalPubPath}, noUpload, false)
	return nil
}

// confirmYes reads one line from cmd's stdin and returns true only for "yes".
func confirmYes(cmd *cobra.Command) (bool, error) {
	reader := bufio.NewReader(cmd.InOrStdin())
	line, _ := reader.ReadString('\n')
	return strings.TrimSpace(line) == "yes", nil
}

// createPreviewLine is the confirmation preview (a one-line summary; the
// stages were already printed by the gate).
func createPreviewLine(in identity.CreateInput, stage1, stage2 tuikit.WizardStageMsg) string {
	_ = stage1
	_ = stage2
	return fmt.Sprintf("create %s: alias %s -> %s:%d, git author %q <%s>", in.Name, in.Alias, in.Hostname, in.Port, in.GitName, in.GitEmail)
}

// runCreateDryRun runs both gate stages against a staged key, prints the two
// outcomes and the four artifact previews, cleans up the staged key, and
// exits 0 having written nothing under ~/.ssh or ~/.gitconfig. The staging
// directory is left clean (the per-verb dry-run contract row for create/clone).
func runCreateDryRun(cmd *cobra.Command, b *realBackend, in identity.CreateInput, id tuikit.DemoIdentity, noUpload bool) error {
	msg1, msg2, err := cliPreWriteGate(b, in, id)
	if err != nil {
		return err
	}

	fmt.Fprintf(cmd.OutOrStdout(), "would create %q (alias %s)\n", in.Name, in.Alias)                                  //nolint:errcheck // best-effort stdout
	fmt.Fprintf(cmd.OutOrStdout(), "  stage 1 (key direct): %s\n", testOutcomeLabel(msg1.Result.Outcome))              //nolint:errcheck // best-effort stdout
	fmt.Fprintf(cmd.OutOrStdout(), "  stage 2 (alias via staged config): %s\n", testOutcomeLabel(msg2.Result.Outcome)) //nolint:errcheck // best-effort stdout

	staged, err := b.stagedKeyFor(in, in.ReuseKeyPath)
	if err != nil {
		return err
	}
	hostBlock, herr := sshconfig.RenderCheckedHostBlock(in.Alias, in.Hostname, in.Port, staged.FinalPrivatePath, in.Provider)
	if herr != nil {
		return fmt.Errorf("gitid: identity create --dry-run: %w", herr)
	}
	fmt.Fprintf(cmd.OutOrStdout(), "  ssh config block:\n%s", indentBlock(hostBlock)) //nolint:errcheck // best-effort stdout

	gitSpec := tuikit.GitSpec{
		Identity:      in.Name,
		Name:          in.GitName,
		Email:         in.GitEmail,
		Strategy:      id.MatchStrategy,
		KeyPath:       staged.FinalPrivatePath,
		PublicKeyPath: staged.FinalPrivatePath + ".pub",
		SSHHost:       in.Alias,
		Provider:      in.Provider,
		GitDir:        id.GitDir,
		// WR-06: mirror the real ceremony's id.ForceSSH — the preview must
		// never claim a rewrite the real write would not perform.
		ForceSSH: id.ForceSSH,
	}
	fmt.Fprintf(cmd.OutOrStdout(), "  fragment (~/.gitconfig.d/%s):\n%s", in.Name, indentBlock(b.GitFragmentPreview(gitSpec))) //nolint:errcheck // best-effort stdout

	includeIf := gitconfig.RenderIncludeIf(in.Name, in.FragmentPath, matchesFor(gitSpec))
	fmt.Fprintf(cmd.OutOrStdout(), "  includeIf block:\n%s", indentBlock(includeIf)) //nolint:errcheck // best-effort stdout

	if staged.PubLine != "" {
		if line, lerr := keygen.AllowedSignersLine(in.GitEmail, staged.PubLine); lerr == nil {
			fmt.Fprintf(cmd.OutOrStdout(), "  allowed_signers:\n  %s\n", line) //nolint:errcheck // best-effort stdout
		}
	}

	// R11/D-06: the upload preview is ADDITIONAL dry-run output, printed
	// BEFORE cleanup so a real (staged, never-final) .pub file exists for
	// planUpload to read — the same temp-sibling pattern
	// uploadRequestFromSpec uses for the TUI wizard's generate path
	// (CR-02/CR-08: the real ~/.ssh stays untouched either way).
	//
	// CR-01 (iteration 2): staged.TempPrivatePath is only a safe staging-dir
	// temp path on the GENERATE path (staged.PrivPEM != nil). On the REUSE
	// path (identity.StageReuse) TempPrivatePath IS the user's real key
	// path, so deriving a ".pub" sibling from it and writing that sibling
	// unconditionally would create a real file under ~/.ssh with no
	// confirmation and no backup — exactly what this dry run promises not
	// to do. Only stage a temp .pub for the generate path; for reuse, only
	// preview against the real .pub if it ALREADY exists on disk (never
	// synthesize it here) — mirroring uploadRequestFromSpec's identical,
	// already-safe `staged.PrivPEM != nil` guard (upload_run.go).
	if staged.PubLine != "" {
		pubPath := staged.FinalPubPath
		if staged.PrivPEM != nil { // generated: a temp sibling in the staging dir only
			tempPub := staged.TempPrivatePath + ".pub"
			pubPath = tempPub
			if !b.deps.PubExists(tempPub) {
				// WR-08: honor the "a write failure here just skips the
				// upload preview" comment for real, instead of ignoring the
				// error and letting the preview report a bogus registration
				// failure for a local file-write problem.
				if werr := b.deps.WritePub(tempPub, staged.PubLine); werr != nil {
					pubPath = ""
				}
			}
		}
		if pubPath != "" && b.deps.PubExists(pubPath) {
			runUploadStep(cmd.OutOrStdout(), b, uploadRequest{Identity: in.Name, Hostname: in.Hostname, PubPath: pubPath}, noUpload, true)
		}
	}

	// The dry-run contract row: the staging directory is cleaned up and
	// nothing is written under ~/.ssh or ~/.gitconfig.
	if in.ReuseKeyPath == "" || (staged.TempPrivatePath != "" && staged.TempPrivatePath != staged.FinalPrivatePath) {
		b.deps.Cleanup(staged)
	}
	return nil
}

// indentBlock indents every line of s by two spaces so multi-line previews
// nest under their label.
func indentBlock(s string) string {
	trimmed := strings.TrimRight(s, "\n")
	if trimmed == "" {
		return "  (empty)\n"
	}
	var b strings.Builder
	for _, line := range strings.Split(trimmed, "\n") {
		b.WriteString("  " + line + "\n")
	}
	return b.String()
}

// testOutcomeLabel renders a tuikit.TestOutcome as the CLI vocabulary the
// dry-run contract table uses.
func testOutcomeLabel(o tuikit.TestOutcome) string {
	switch o {
	case tuikit.TestOutcomePass:
		return "PASS"
	case tuikit.TestOutcomeReachableNotUploaded:
		return "reachable-not-uploaded"
	default:
		return "failure"
	}
}

// outcomeLabel renders a tester.Outcome as the CLI vocabulary (the rotate/
// new-key re-test exit-code message).
func outcomeLabel(o tester.Outcome) string {
	switch o {
	case tester.PASS:
		return "PASS"
	case tester.ReachableNotUploaded:
		return "reachable-not-uploaded"
	default:
		return "failure"
	}
}
