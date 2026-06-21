package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/castocolina/gitid/internal/clipboard"
	"github.com/castocolina/gitid/internal/doctor/checks"
	"github.com/castocolina/gitid/internal/filewriter"
	"github.com/castocolina/gitid/internal/gitconfig"
	"github.com/castocolina/gitid/internal/identity"
	"github.com/castocolina/gitid/internal/keygen"
	"github.com/castocolina/gitid/internal/platform"
	"github.com/castocolina/gitid/internal/sshconfig"
	"github.com/castocolina/gitid/internal/tester"
)

// addFlags holds non-interactive flag values for `gitid identity add` (D-09).
// A non-empty field skips the corresponding prompt.
type addFlags struct {
	name     string // --name: identity name
	gitdir   string // --gitdir: gitdir match value
	url      string // --url: hasconfig URL pattern (bare; buildMatches prepends "remote.*.url:")
	provider string // --provider: provider name
	match    string // --match: gitdir|hasconfig|both (non-interactive strategy selector, D-10 parity)
}

// newAddCmd builds `gitid identity add` (create-new mode). The handler is thin:
// it gathers input, builds identity.Deps from the real internal packages, calls
// identity.Create, and prints. All orchestration logic lives in
// internal/identity.Create (no business logic in cmd/).
func newAddCmd() *cobra.Command {
	var dryRun bool
	var flags addFlags
	cmd := &cobra.Command{
		Use:   "add",
		Short: "Create a new Git identity (key, SSH config, gitconfig, allowed_signers)",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runIdentityAdd(cmd.InOrStdin(), cmd.OutOrStdout(), dryRun, flags, buildDeps)
		},
	}
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "preview the four artifacts without writing anything (SAFE-03)")
	cmd.Flags().StringVar(&flags.name, "name", "", "identity name (skips name prompt; D-09)")
	cmd.Flags().StringVar(&flags.gitdir, "gitdir", "", "gitdir match value (skips gitdir prompt; D-09)")
	cmd.Flags().StringVar(&flags.url, "url", "", "hasconfig URL pattern (skips URL prompt; D-09)")
	cmd.Flags().StringVar(&flags.provider, "provider", "", "provider name (skips provider prompt; D-09)")
	cmd.Flags().StringVar(&flags.match, "match", "", "match strategy: gitdir|hasconfig|both (non-interactive parity; D-10)")
	//nolint:errcheck // completion registration failure is non-fatal (cobra ignores it gracefully)
	_ = cmd.RegisterFlagCompletionFunc("match", func(_ *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
		return []string{"gitdir", "hasconfig", "both"}, cobra.ShellCompDirectiveNoFileComp
	})
	return cmd
}

// runIdentityAdd is the create-new orchestration handler. It probes the
// algorithm (D-14 stop on none), gathers inputs via prompts (D-05), builds the
// four-writer Deps wiring keygen.WriteAllowedSigners and
// gitconfig.SetAllowedSignersFile, calls identity.Create, prints the test
// command+output (TEST-03) and the unified four-artifact preview, asks one
// explicit confirmation (skipped under --dry-run, SAFE-03), and on confirm loads
// the key into the agent (ssh-add, D-08) and prints upload steps.
func runIdentityAdd(in io.Reader, out io.Writer, dryRun bool, flags addFlags, depsFor func(io.Writer) identity.Deps) error {
	supported, err := platform.ProbeKeyTypes()
	if err != nil {
		return fmt.Errorf("identity add: probing key algorithms: %w", err)
	}
	algo, warned, err := platform.SelectAlgorithm(supported)
	if err != nil {
		// D-14: no supported algorithm — stop with the actionable install hint.
		fp(out, err.Error()+"\n")
		return err
	}
	if warned {
		fp(out, fmt.Sprintf("Note: ed25519 unavailable; using %s instead.\n", algo))
	}

	reader := bufio.NewReader(in)

	// D-10: the user chooses one of three create modes at the start.
	mode := selectMode(reader, out)

	switch mode {
	case modeReuse:
		return runReuse(reader, out, algo, dryRun, depsFor)
	case modeAddAccount:
		return runAddAccount(reader, out, dryRun, depsFor)
	default: // modeCreateNew
		return runCreateNew(reader, out, algo, dryRun, flags, depsFor)
	}
}

// createMode enumerates the three D-10 create modes offered by `identity add`.
type createMode int

const (
	modeCreateNew createMode = iota
	modeReuse
	modeAddAccount
)

// selectMode prompts for the create mode (D-10), defaulting to create-new. It
// accepts a numeric choice (1/2/3) or a keyword (new/reuse/add-account).
func selectMode(r *bufio.Reader, out io.Writer) createMode {
	fp(out, "Create mode:\n")
	fp(out, "  1) new          — generate a fresh key (default)\n")
	fp(out, "  2) reuse        — reuse an existing private key\n")
	fp(out, "  3) add-account  — add an alias for an existing identity\n")
	choice := strings.ToLower(prompt(r, out, "Choose mode", "1"))
	switch choice {
	case "2", "reuse", "reuse-existing-key":
		return modeReuse
	case "3", "add-account", "add", "alias":
		return modeAddAccount
	default:
		return modeCreateNew
	}
}

// runCreateNew is the create-new orchestration (D-01..D-06):
//
//  1. gather inputs (no pre-test confirm prompt — D-02 removed)
//  2. Generate the key pair directly to ~/.ssh (D-01)
//  3. copy .pub to clipboard + print upload instructions
//  4. --dry-run: render previews and return (no key generated under dry-run — Q1)
//  5. runCreateLoop: auth-gated loop (retry/skip/quit) — D-03..D-06
//  6. on persist: call identity.PersistAll; print backup paths + resolved result
func runCreateNew(reader *bufio.Reader, out io.Writer, algo string, dryRun bool, flags addFlags, depsFor func(io.Writer) identity.Deps) error {
	input, err := gatherCreateInput(reader, out, algo, flags)
	if err != nil {
		return err
	}

	deps := depsFor(out)

	// Q1: under --dry-run, skip key generation entirely and return previews only.
	if dryRun {
		// Build a synthetic staged key for preview rendering (no real key generated).
		syntheticStaged := identity.StagedKey{
			FinalPrivatePath: "~/.ssh/id_" + input.Algo + "_" + input.Name,
			FinalPubPath:     "~/.ssh/id_" + input.Algo + "_" + input.Name + ".pub",
			PubLine:          "(key will be generated)",
		}
		res := identity.RenderPreviews(input, syntheticStaged)
		printPreview(out, res)
		fp(out, "\n--dry-run: no files were written.\n")
		return nil
	}

	// D-01: Generate the key pair directly to ~/.ssh immediately.
	staged, err := deps.Generate(input)
	if err != nil {
		return fmt.Errorf("identity add: generating key: %w", err)
	}

	// Copy .pub to clipboard (best-effort; failure is non-fatal).
	if cerr := deps.CopyPub(staged.PubLine); cerr != nil {
		_ = cerr
	}
	printPubForManualCopy(out, staged.PubLine)
	fp(out, "\n"+uploadInstructions(input.Provider)+"\n")

	// D-03..D-06: auth-gated loop; loops until PASS, skip+confirm, or quit.
	persist, _, err := runCreateLoop(reader, out, input, staged, deps)
	if err != nil {
		return err
	}
	if !persist {
		// D-04: quit — key stays in ~/.ssh, no config written.
		return nil
	}

	// D-16: check for overlapping match conditions before persisting.
	// Build a prospective account from the gathered input so DetectOverlaps can
	// compare it against existing on-disk identities.
	prospective := identity.Account{
		Name:    input.Name,
		Matches: input.Matches,
	}
	if !warnOverlapAndConfirm(reader, out, prospective, loadExistingAccounts()) {
		fp(out, "Add cancelled; no config files were written.\n")
		return nil
	}

	// Persist all four config artifacts (D-03/D-05).
	res, err := identity.PersistAll(input, staged, deps)
	if err != nil {
		return err
	}
	printPreview(out, res)
	printBackupPaths(out, res)
	loadKeyIntoAgent(out, res.Key.PrivatePath)
	printResolved(out, res)
	return nil
}

// runReuse is the reuse-existing-key orchestration (IDENT-02): gather inputs plus
// the existing private-key path, call identity.Reuse (which derives the .pub when
// absent), and print results.
func runReuse(reader *bufio.Reader, out io.Writer, algo string, dryRun bool, depsFor func(io.Writer) identity.Deps) error {
	input, err := gatherCreateInput(reader, out, algo, addFlags{})
	if err != nil {
		return err
	}
	existingKey := prompt(reader, out, "Existing private key path", "")
	if strings.TrimSpace(existingKey) == "" {
		return fmt.Errorf("identity add: existing private key path is required for reuse")
	}

	deps := depsFor(out)
	res, err := identity.Reuse(input, existingKey, deps)
	if err != nil {
		return err
	}

	printPreWrite(out, res.PreWrite)
	printPreview(out, res)

	if dryRun {
		fp(out, "\n--dry-run: no files were written.\n")
		return nil
	}
	if !res.PreWriteOnly {
		loadKeyIntoAgent(out, res.Key.PrivatePath)
		printResolved(out, res)
	}
	fp(out, "\n"+uploadInstructions(input.Provider)+"\n")
	printPubForManualCopy(out, res.Key.PubLine)
	return nil
}

// runAddAccount is the add-account/alias orchestration (IDENT-06): gather the
// existing identity's details plus the new provider/alias, call
// identity.AddAccount (sharing the existing key), and print results.
func runAddAccount(reader *bufio.Reader, out io.Writer, dryRun bool, depsFor func(io.Writer) identity.Deps) error {
	existing, newProvider, newAlias, err := gatherAddAccount(reader, out)
	if err != nil {
		return err
	}

	// Under --dry-run we only render the alias preview and perform no write: since
	// AddAccount is a confirmed write path, we render the SSH/includeIf previews
	// directly rather than invoking it (SAFE-03 dry-run is strictly read-only).
	if dryRun {
		fp(out, "\n=== Preview: add-account alias ===\n")
		fp(out, "--- ~/.ssh/config (Host block) ---\n")
		fp(out, sshconfig.RenderHostBlock(newAlias, existing.Hostname, existing.Port, existing.KeyPath, existing.Provider))
		fp(out, "--- ~/.gitconfig (includeIf) ---\n")
		fp(out, gitconfig.RenderIncludeIf(existing.Name, existing.FragmentPath, existing.Matches)+"\n")
		fp(out, "\n--dry-run: no files were written.\n")
		return nil
	}

	// AddAccount performs a confirmed write; gate it behind one explicit consent
	// (SAFE-03).
	if !confirm(reader, out, "Add this account/alias now?") {
		fp(out, "Add-account cancelled; no files were written.\n")
		return nil
	}

	deps := depsFor(out)
	res, err := identity.AddAccount(existing, newProvider, newAlias, deps)
	if err != nil {
		return err
	}

	printPreWrite(out, res.PreWrite)
	printPreview(out, res)
	printResolved(out, res)
	fp(out, "\n"+uploadInstructions(newProvider)+"\n")
	return nil
}

// gatherAddAccount collects the existing identity's details and the new alias.
func gatherAddAccount(r *bufio.Reader, out io.Writer) (identity.Account, string, string, error) {
	name := sanitizeName(prompt(r, out, "Existing identity name", ""))
	if name == "" {
		return identity.Account{}, "", "", fmt.Errorf("identity add: existing identity name is required")
	}
	if !identityNameRe.MatchString(name) {
		return identity.Account{}, "", "", fmt.Errorf("identity add: invalid identity name %q (allowed: letters, digits, '.', '_', '-')", name)
	}
	gitName := prompt(r, out, "Git user.name", "")
	gitEmail := prompt(r, out, "Git user.email", "")
	keyPath := prompt(r, out, "Existing private key path", "")
	if strings.TrimSpace(keyPath) == "" {
		return identity.Account{}, "", "", fmt.Errorf("identity add: existing private key path is required for add-account")
	}
	newProvider := prompt(r, out, "New provider (github/gitlab)", "gitlab")
	if err := identity.ValidateProvider(newProvider); err != nil {
		return identity.Account{}, "", "", fmt.Errorf("identity add: %w", err)
	}
	newAlias := prompt(r, out, "New host alias", identity.DefaultAlias(name, newProvider))
	hostname := prompt(r, out, "Hostname", defaultHostname(newProvider))
	port := promptPort(r, out, "Port", 443)
	matchDir := prompt(r, out, "Match gitdir", "~/git/"+name+"/")

	home, err := os.UserHomeDir()
	if err != nil {
		return identity.Account{}, "", "", fmt.Errorf("identity add: resolving home dir: %w", err)
	}

	acct := identity.Account{
		Name:               name,
		GitName:            gitName,
		GitEmail:           gitEmail,
		Hostname:           hostname,
		Port:               port,
		KeyPath:            keyPath,
		PubPath:            keyPath + ".pub",
		Matches:            []gitconfig.Match{{Kind: gitconfig.MatchGitdir, Value: matchDir}},
		FragmentPath:       filepath.Join(home, ".gitconfig.d", name),
		GitconfigPath:      filepath.Join(home, ".gitconfig"),
		SSHConfigPath:      filepath.Join(home, ".ssh", "config"),
		AllowedSignersPath: filepath.Join(home, ".ssh", "allowed_signers"),
	}
	return acct, newProvider, newAlias, nil
}

// fp writes s to out, ignoring the write error: out is the command's stdout,
// where a write failure is neither recoverable nor actionable.
func fp(out io.Writer, s string) {
	_, _ = io.WriteString(out, s)
}

// gatherCreateInput collects the create-new inputs via interactive prompts with
// sensible defaults shown (D-05). The host alias is pre-selected to
// <identity>.<provider> (D-12) and the match defaults to gitdir:~/git/<id>/
// (D-13). The old pre-test confirm prompt (D-02) is removed
// (D-02); the persist decision belongs to runCreateLoop after an authenticated
// PASS or explicit skip+confirm (D-03/D-05).
// gatherCreateInput collects the create-new inputs via interactive prompts with
// sensible defaults shown (D-05). Non-empty flag values skip the corresponding
// prompt (D-09). The match strategy picker replaces the single gitdir prompt (D-07).
// The host alias is pre-selected to <identity>.<provider> (D-12).
// The old pre-test confirm prompt (D-02) is removed; the persist decision belongs
// to runCreateLoop after an authenticated PASS or explicit skip+confirm (D-03/D-05).
func gatherCreateInput(r *bufio.Reader, out io.Writer, algo string, flags addFlags) (identity.CreateInput, error) {
	// Identity name: flag-or-prompt (D-09).
	var name string
	if flags.name != "" {
		name = sanitizeName(flags.name)
	} else {
		name = sanitizeName(prompt(r, out, "Identity name", ""))
	}
	if name == "" {
		return identity.CreateInput{}, fmt.Errorf("identity add: identity name is required")
	}
	if !identityNameRe.MatchString(name) {
		return identity.CreateInput{}, fmt.Errorf("identity add: invalid identity name %q (allowed: letters, digits, '.', '_', '-')", name)
	}
	gitName := prompt(r, out, "Git user.name", "")
	gitEmail := prompt(r, out, "Git user.email", "")

	// Provider: flag-or-prompt (D-09).
	var provider string
	if flags.provider != "" {
		provider = flags.provider
	} else {
		provider = prompt(r, out, "Provider (github/gitlab)", "github")
	}
	if err := identity.ValidateProvider(provider); err != nil {
		return identity.CreateInput{}, fmt.Errorf("identity add: %w", err)
	}

	alias := prompt(r, out, "Host alias", identity.DefaultAlias(name, provider))
	hostname := prompt(r, out, "Hostname", defaultHostname(provider))
	port := promptPort(r, out, "Port", 443)

	// Match strategy: flag-or-prompt (D-07, D-09, D-10).
	// Priority (most-specific wins):
	//   1. Both --gitdir and --url given → both strategy (explicit flag pair).
	//   2. Only --gitdir given → gitdir strategy.
	//   3. Only --url given → url strategy.
	//   4. --match given (and no --gitdir/--url) → derive defaults from flag.
	//   5. Otherwise → interactive picker (what the e2e stubs drive).
	var matches []gitconfig.Match
	gitdirDefault := "~/git/" + name + "/"
	urlDefault := defaultURLPattern(hostname, name)
	switch {
	case flags.gitdir != "" && flags.url != "":
		matches = buildMatches("3", flags.gitdir, flags.url)
	case flags.gitdir != "":
		matches = buildMatches("1", flags.gitdir, "")
	case flags.url != "":
		matches = buildMatches("2", "", flags.url)
	case flags.match != "":
		// --match flag: non-interactive parity surface (D-10). Derive strategy
		// number via matchFromFlag, then supply sensible defaults for missing
		// --gitdir/--url so the caller does not need to provide both.
		stratNum, ferr := matchFromFlag(flags.match)
		if ferr != nil {
			return identity.CreateInput{}, ferr
		}
		matches = buildMatches(stratNum, gitdirDefault, urlDefault)
	default:
		matches = promptMatchStrategy(r, out, gitdirDefault, urlDefault)
	}

	passphrase := prompt(r, out, "Passphrase (empty for none)", "")

	home, err := os.UserHomeDir()
	if err != nil {
		return identity.CreateInput{}, fmt.Errorf("identity add: resolving home dir: %w", err)
	}

	return identity.CreateInput{
		Name:               name,
		GitName:            gitName,
		GitEmail:           gitEmail,
		Provider:           provider,
		Algo:               algo,
		Alias:              alias,
		Hostname:           hostname,
		Port:               port,
		Passphrase:         passphrase,
		Matches:            matches,
		FragmentPath:       filepath.Join(home, ".gitconfig.d", name),
		GitconfigPath:      filepath.Join(home, ".gitconfig"),
		SSHConfigPath:      filepath.Join(home, ".ssh", "config"),
		AllowedSignersPath: filepath.Join(home, ".ssh", "allowed_signers"),
		GlobalBlock:        sshconfig.RenderGlobalBlock(platform.CurrentOS()),
	}, nil
}

// runCreateLoop drives the auth-gated retry/skip/quit loop (D-03..D-06).
// It calls deps.PreWrite on each iteration; on tester.PASS it returns
// (persist=true, skipConfirmed=false) immediately (D-03 auto-persist, no extra
// prompt). On [s] skip+confirm it returns (true, true) after the explicit typed
// confirm and "not yet authenticated" warning (D-05). On [q] quit it prints the
// key-kept-at-path + doctor-orphan note and returns (false, false, nil) (D-04).
// Default ([r] or empty) loops.
func runCreateLoop(r *bufio.Reader, out io.Writer, in identity.CreateInput, staged identity.StagedKey, deps identity.Deps) (persist bool, skipConfirmed bool, err error) {
	for {
		pre := deps.PreWrite(staged.FinalPrivatePath, in.Hostname, in.Port)
		printPreWrite(out, pre)
		if pre.Outcome == tester.PASS {
			// D-03: authenticated PASS → auto-persist, no extra prompt.
			return true, false, nil
		}
		fp(out, "\n[r] retry  [s] skip-&-write (offline/upload-later)  [q] quit\n")
		choice := strings.ToLower(strings.TrimSpace(prompt(r, out, "Choice", "r")))
		switch choice {
		case "s", "skip":
			// D-05: require explicit typed confirm before persisting without PASS.
			fp(out, "Warning: key is not yet authenticated. You must upload the key and verify authentication before using this identity.\n")
			if confirm(r, out, "Write config artifacts without authenticated PASS?") {
				return true, true, nil
			}
			// Declined → loop continues.
		case "q", "quit":
			// D-04: keep key in ~/.ssh, no config write.
			fp(out, fmt.Sprintf("Quit. Key kept at %s. Run 'gitid doctor' when ready to write config.\n", staged.FinalPrivatePath))
			return false, false, nil
		default:
			// "r" or empty → retry (loop continues).
		}
	}
}

// warnOverlapAndConfirm checks whether adding prospective to the existing identity
// list would create overlapping match conditions (D-16). If overlaps are detected,
// it prints a warning naming the conflicting identities and the last-wins note, then
// asks the user to explicitly confirm before proceeding. Returns true when safe to
// proceed (no overlaps, or user confirmed), false when the user declined.
// Called by add's runCreateNew (before PersistAll) and update's runIdentityUpdate
// (before Update) so both write paths share the same detector.
func warnOverlapAndConfirm(r *bufio.Reader, out io.Writer, prospective identity.Account, existing []identity.Account) bool {
	// Drop any on-disk account that shares the prospective's name: re-creating or
	// rewriting the same identity is not a real overlap, and including it would
	// emit a confusing self-referential "X and X overlap" warning. (The update
	// path already pre-excludes by name; doing it here covers the add path too.)
	all := make([]identity.Account, 0, len(existing)+1)
	for _, a := range existing {
		if a.Name != prospective.Name {
			all = append(all, a)
		}
	}
	all = append(all, prospective)
	pairs := checks.DetectOverlaps(all)
	// Filter to only pairs that involve the prospective identity.
	var relevant []checks.OverlapPair
	for _, p := range pairs {
		if p.A == prospective.Name || p.B == prospective.Name {
			relevant = append(relevant, p)
		}
	}
	if len(relevant) == 0 {
		return true // no overlap — safe to proceed
	}
	fp(out, "\nWarning: overlapping match conditions detected:\n")
	for _, p := range relevant {
		other := p.B
		if p.B == prospective.Name {
			other = p.A
		}
		fp(out, fmt.Sprintf("  %q and %q: %s — git will use last-written-wins for conflicting keys.\n",
			prospective.Name, other, p.Kind))
		fp(out, fmt.Sprintf("  Detail: %s\n", p.Detail))
	}
	fp(out, "  Tip: narrow one of the match conditions (gitdir path or URL pattern) to avoid ambiguity.\n")
	return confirm(r, out, "Proceed with overlapping match conditions?")
}

// loadExistingAccounts reads and reconstructs all identities from disk.
// Returns an empty slice (not an error) when no config files exist yet.
func loadExistingAccounts() []identity.Account {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil
	}
	sshBytes, _ := os.ReadFile(filepath.Join(home, ".ssh", "config")) //nolint:gosec // gitid-managed path (G304)
	gcBytes, _ := os.ReadFile(filepath.Join(home, ".gitconfig"))      //nolint:gosec // gitid-managed path (G304)
	accounts, _ := identity.Reconstruct(sshBytes, gcBytes, gitconfig.ReadFragment)
	return accounts
}

// buildDeps wires identity.Deps from the real internal packages, including the
// FOURTH writer keygen.WriteAllowedSigners and the global pointer
// gitconfig.SetAllowedSignersFile inside the gitconfig writer.
func buildDeps(_ io.Writer) identity.Deps {
	return identity.Deps{
		// D-01: Generate writes the key pair directly to ~/.ssh immediately.
		// TempPrivatePath == FinalPrivatePath (no temp staging); Cleanup is a
		// guaranteed no-op. This replaces the old temp-stage→promote behavior
		// (BUG-4 fix: the key is now on disk and uploadable before the gate runs).
		Generate: func(in identity.CreateInput) (identity.StagedKey, error) {
			home, herr := os.UserHomeDir()
			if herr != nil {
				return identity.StagedKey{}, herr
			}
			sshDir := filepath.Join(home, ".ssh")
			// Ensure ~/.ssh exists and has the correct permission (0700) before
			// writing any key material. filewriter.EnsureDir is idempotent.
			if eerr := filewriter.EnsureDir(sshDir, 0o700); eerr != nil { //nolint:gosec // creating gitid-managed ~/.ssh dir (G301)
				return identity.StagedKey{}, fmt.Errorf("identity add: ensuring ~/.ssh exists: %w", eerr)
			}
			finalPriv, finalPub := keygen.KeyPaths(sshDir, in.Algo, in.Name)
			mat, gerr := keygen.GenerateMaterial(keygen.Params{
				Algo:       in.Algo,
				Identity:   in.Name,
				Comment:    in.Name + "@gitid",
				Passphrase: in.Passphrase,
			})
			if gerr != nil {
				return identity.StagedKey{}, gerr
			}
			// Write private key directly to final ~/.ssh path (D-01, T-05.5-08:
			// 0600 via filewriter chokepoint — never os.WriteFile directly).
			if _, werr := filewriter.Write(finalPriv, mat.PrivPEM, 0o600); werr != nil { //nolint:gosec // gitid-managed final path (G306)
				return identity.StagedKey{}, fmt.Errorf("identity add: writing private key to ~/.ssh: %w", werr)
			}
			// Write public key (0644).
			if _, werr := filewriter.Write(finalPub, []byte(mat.PubLine), 0o644); werr != nil {
				return identity.StagedKey{}, fmt.Errorf("identity add: writing public key to ~/.ssh: %w", werr)
			}
			return identity.StagedKey{
				TempPrivatePath:  finalPriv, // == FinalPrivatePath (D-01: no temp staging)
				FinalPrivatePath: finalPriv,
				FinalPubPath:     finalPub,
				PubLine:          mat.PubLine,
				PrivPEM:          mat.PrivPEM,
			}, nil
		},
		PersistKey: func(staged identity.StagedKey) (identity.KeyResult, error) {
			// D-01: key was already written by Generate; PersistKey is a no-op
			// for the create-new flow. Still called for Rotate which uses runPipeline.
			if staged.PrivPEM == nil {
				return identity.KeyResult{
					PrivatePath: staged.FinalPrivatePath,
					PubPath:     staged.FinalPubPath,
					PubLine:     staged.PubLine,
				}, nil
			}
			if _, werr := filewriter.Write(staged.FinalPrivatePath, staged.PrivPEM, 0o600); werr != nil {
				return identity.KeyResult{}, fmt.Errorf("identity add: writing final private key: %w", werr)
			}
			if _, werr := filewriter.Write(staged.FinalPubPath, []byte(staged.PubLine), 0o644); werr != nil {
				return identity.KeyResult{}, fmt.Errorf("identity add: writing final public key: %w", werr)
			}
			return identity.KeyResult{
				PrivatePath: staged.FinalPrivatePath,
				PubPath:     staged.FinalPubPath,
				PubLine:     staged.PubLine,
			}, nil
		},
		Cleanup: func(_ identity.StagedKey) {
			// D-01: no temp dir; Cleanup is an unconditional no-op.
		},
		CopyPub: clipboard.Copy,
		PreWrite: func(keyPath, hostname string, port int) tester.Result {
			return tester.PreWrite(keyPath, hostname, port)
		},
		WriteSSH: func(accountName, hostBlock, globalBlock string) (string, error) {
			home, herr := os.UserHomeDir()
			if herr != nil {
				return "", herr
			}
			return sshconfig.Write(filepath.Join(home, ".ssh", "config"), accountName, hostBlock, globalBlock)
		},
		WriteGitconfig: func(id, fragmentPath, allowedSignersPath string, matches []gitconfig.Match) (string, error) {
			home, herr := os.UserHomeDir()
			if herr != nil {
				return "", herr
			}
			gitconfigPath := filepath.Join(home, ".gitconfig")
			backup, werr := gitconfig.WriteIncludeIf(gitconfigPath, id, fragmentPath, matches)
			if werr != nil {
				return backup, werr
			}
			// Global pointer so SSH-signed commits verify against allowed_signers.
			if serr := gitconfig.SetAllowedSignersFile(gitconfigPath, allowedSignersPath); serr != nil {
				return backup, serr
			}
			return backup, nil
		},
		WriteFragment: func(fragPath, name, email, signingKeyPath string, signing bool) error {
			return gitconfig.WriteFragment(fragPath, name, email, signingKeyPath, signing)
		},
		WriteAllowedSigners: keygen.WriteAllowedSigners,
		Resolved:            tester.Resolved,
		PubExists: func(pubPath string) bool {
			_, err := os.Stat(pubPath)
			return err == nil
		},
		DerivePub: keygen.DerivePublicKey,
		WritePub: func(pubPath, pubLine string) error {
			// Derived .pub is public material, written 0644 via the filewriter
			// chokepoint (T-02-28).
			_, werr := filewriter.Write(pubPath, []byte(pubLine), 0o644)
			return werr
		},
	}
}

// defaultHostname returns the conventional SSH hostname for the known providers
// (port 443 alt-ssh endpoints), falling back to a sensible guess otherwise.
func defaultHostname(provider string) string {
	switch strings.ToLower(provider) {
	case "github":
		return "ssh.github.com"
	case "gitlab":
		return "altssh.gitlab.com"
	default:
		return provider + ".com"
	}
}

// loadKeyIntoAgent runs ssh-add (arg-slice, no shell) to load the new key into
// the agent; on macOS it adds --apple-use-keychain (D-08). A missing ssh-add is
// a warn-and-continue, never a hard failure.
func loadKeyIntoAgent(out io.Writer, keyPath string) {
	if _, err := exec.LookPath("ssh-add"); err != nil {
		fp(out, "Warning: ssh-add not found; skipping agent load. Add the key manually.\n")
		return
	}
	args := []string{}
	if runtime.GOOS == "darwin" {
		args = append(args, "--apple-use-keychain")
	}
	args = append(args, keyPath)
	cmd := exec.Command("ssh-add", args...) //nolint:gosec // arg-slice form, no shell; keyPath is a gitid-managed path (G204)
	if combined, err := cmd.CombinedOutput(); err != nil {
		fp(out, fmt.Sprintf("Warning: ssh-add failed (continuing): %s\n", strings.TrimSpace(string(combined))))
	}
}

func printPreWrite(out io.Writer, r tester.Result) {
	fp(out, "Pre-write connectivity test:\n")
	fp(out, fmt.Sprintf("$ %s\n%s\n", r.Command, strings.TrimRight(r.Output, "\n")))
}

func printPreview(out io.Writer, res identity.CreateResult) {
	fp(out, "\n=== Preview: four coordinated artifacts ===\n")
	fp(out, "--- ~/.ssh/config (Host block) ---\n")
	fp(out, res.SSHPreview)
	fp(out, "--- ~/.gitconfig (includeIf) ---\n")
	fp(out, res.GitconfigPreview+"\n")
	fp(out, "--- gitconfig fragment ---\n")
	fp(out, res.FragmentPreview)
	fp(out, "--- ~/.ssh/allowed_signers ---\n")
	fp(out, res.AllowedSignersPreview)
}

func printResolved(out io.Writer, res identity.CreateResult) {
	fp(out, "\nResolved test:\n")
	fp(out, fmt.Sprintf("$ %s\n%s\n", res.ResolvedTest.Command, strings.TrimRight(res.ResolvedTest.Output, "\n")))
	fp(out, fmt.Sprintf("  user=%s hostname=%s port=%s identitiesonly=%s\n",
		res.Resolved.User, res.Resolved.Hostname, res.Resolved.Port, res.Resolved.IdentitiesOnly))
}

func printPubForManualCopy(out io.Writer, pubLine string) {
	fp(out, "Public key (also copied to your clipboard):\n")
	fp(out, strings.TrimRight(pubLine, "\n")+"\n")
}

// printBackupPaths prints the backup file paths returned by the four writers
// after a confirmed write (CLAUDE.md safe-write invariant, WR-05).
func printBackupPaths(out io.Writer, res identity.CreateResult) {
	if res.SSHBackup != "" {
		fp(out, fmt.Sprintf("  SSH config backup:      %s\n", res.SSHBackup))
	}
	if res.GitconfigBackup != "" {
		fp(out, fmt.Sprintf("  gitconfig backup:       %s\n", res.GitconfigBackup))
	}
	if res.AllowedSignersBackup != "" {
		fp(out, fmt.Sprintf("  allowed_signers backup: %s\n", res.AllowedSignersBackup))
	}
}

// prompt reads a single line, showing def as the default when the user enters
// an empty value.
func prompt(r *bufio.Reader, out io.Writer, label, def string) string {
	if def != "" {
		fp(out, fmt.Sprintf("%s [%s]: ", label, def))
	} else {
		fp(out, fmt.Sprintf("%s: ", label))
	}
	line, _ := r.ReadString('\n')
	line = strings.TrimSpace(line)
	if line == "" {
		return def
	}
	return line
}

func promptPort(r *bufio.Reader, out io.Writer, label string, def int) int {
	s := prompt(r, out, label, strconv.Itoa(def))
	if n, err := strconv.Atoi(s); err == nil && n > 0 {
		return n
	}
	return def
}

func confirm(r *bufio.Reader, out io.Writer, label string) bool {
	fp(out, fmt.Sprintf("%s [y/N]: ", label))
	line, _ := r.ReadString('\n')
	line = strings.ToLower(strings.TrimSpace(line))
	return line == "y" || line == "yes"
}
