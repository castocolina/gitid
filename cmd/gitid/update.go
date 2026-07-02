package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/castocolina/gitid/internal/gitconfig"
	"github.com/castocolina/gitid/internal/identity"
	"github.com/castocolina/gitid/internal/keygen"
	"github.com/castocolina/gitid/internal/sshconfig"
	"github.com/castocolina/gitid/internal/tester"
)

// updateFlags holds non-interactive flag values for `gitid identity update` (D-09).
// A non-empty field skips the corresponding prompt.  --name is intentionally
// absent: name stays positional per Q2 (name is immutable; positional arg suffices).
type updateFlags struct {
	gitdir   string // --gitdir: new gitdir match value (skips gitdir prompt)
	url      string // --url: new hasconfig URL pattern (bare; buildMatches prepends "remote.*.url:")
	provider string // --provider: overwrite the provider marker (flag-only; no interactive prompt — Q3)
	match    string // --match: gitdir|hasconfig|both (non-interactive strategy selector, D-10 parity)
}

// newUpdateCmd builds `gitid identity update <name>` (IDENT-04). The handler is
// thin: it validates the name, loads the current identity via reconstruction,
// prompts for edits with pre-filled current values, previews, confirms, and calls
// identity.Update. All orchestration logic lives in internal/identity.Update.
func newUpdateCmd() *cobra.Command {
	var dryRun bool
	var flags updateFlags
	cmd := &cobra.Command{
		Use:   "update <name>",
		Short: "Update an existing Git identity's fields (email, signing, alias, port, match strategy — name immutable)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runIdentityUpdate(cmd.InOrStdin(), cmd.OutOrStdout(), args[0], dryRun, flags, buildUpdateDeps)
		},
	}
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "preview the update without writing anything (SAFE-03)")
	cmd.Flags().StringVar(&flags.gitdir, "gitdir", "", "new gitdir match value (skips gitdir prompt; D-09)")
	cmd.Flags().StringVar(&flags.url, "url", "", "new hasconfig URL pattern (skips URL prompt; D-09)")
	cmd.Flags().StringVar(&flags.provider, "provider", "", "overwrite the provider marker in the SSH Host block (D-11/Q3; flag-only, no interactive prompt)")
	cmd.Flags().StringVar(&flags.match, "match", "", "match strategy: gitdir|hasconfig|both (non-interactive parity; D-10)")
	//nolint:errcheck // completion registration failure is non-fatal (cobra ignores it gracefully)
	_ = cmd.RegisterFlagCompletionFunc("match", func(_ *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
		return []string{"gitdir", "hasconfig", "both"}, cobra.ShellCompDirectiveNoFileComp
	})
	return cmd
}

// runIdentityUpdate is the update orchestration handler. It validates name,
// reconstructs the current identity from disk, prompts for edits pre-filled with
// current values, previews, confirms (unless --dry-run), and calls identity.Update.
func runIdentityUpdate(in io.Reader, out io.Writer, name string, dryRun bool, flags updateFlags, depsFor func(io.Writer) identity.UpdateDeps) error {
	name = sanitizeName(name)
	if name == "" {
		return fmt.Errorf("identity update: identity name is required")
	}
	if !identityNameRe.MatchString(name) {
		return fmt.Errorf("identity update: invalid identity name %q (allowed: letters, digits, '.', '_', '-')", name)
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("identity update: resolving home dir: %w", err)
	}

	sshConfigPath := filepath.Join(home, ".ssh", "config")
	gitconfigPath := filepath.Join(home, ".gitconfig")

	sshBytes, err := os.ReadFile(sshConfigPath) //nolint:gosec // gitid-managed path
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("identity update: reading %s: %w", sshConfigPath, err)
	}
	gcBytes, err := os.ReadFile(gitconfigPath) //nolint:gosec // gitid-managed path
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("identity update: reading %s: %w", gitconfigPath, err)
	}

	accounts, err := identity.Reconstruct(sshBytes, gcBytes, gitconfig.ReadFragment)
	if err != nil {
		return fmt.Errorf("identity update: reconstructing identities: %w", err)
	}

	// Find the account matching the requested name.
	var existing identity.Account
	found := false
	for _, a := range accounts {
		if a.Name == name {
			existing = a
			found = true
			break
		}
	}
	if !found {
		return fmt.Errorf("identity update: identity %q not found (run 'gitid identity list' to see all identities)", name)
	}

	// Fill the gitid-managed paths from HOME (in case reconstruction didn't populate
	// them, mirror the pattern from rotate.go gatherRotateAccount).
	if existing.FragmentPath == "" {
		existing.FragmentPath = filepath.Join(home, ".gitconfig.d", name)
	}
	if existing.GitconfigPath == "" {
		existing.GitconfigPath = gitconfigPath
	}
	if existing.SSHConfigPath == "" {
		existing.SSHConfigPath = sshConfigPath
	}
	if existing.AllowedSignersPath == "" {
		existing.AllowedSignersPath = filepath.Join(home, ".ssh", "allowed_signers")
	}
	if existing.KeyPath == "" {
		existing.KeyPath = filepath.Join(home, ".ssh", "id_ed25519_"+name)
	}
	if existing.PubPath == "" {
		existing.PubPath = existing.KeyPath + ".pub"
	}

	// Detect whether the current identity has signing enabled by checking if
	// the fragment has gpg.format set.
	currentSigning := false
	frag, ferr := gitconfig.ReadFragment(existing.FragmentPath)
	if ferr == nil && !frag.Missing {
		currentSigning = frag.GPGFormat == "ssh"
	}

	reader := bufio.NewReader(in)

	// Prompt for all editable fields, pre-filled with current values.
	fp(out, fmt.Sprintf("\nUpdating identity: %s (name is immutable — D-04)\n\n", name))

	edited := existing
	edited.GitName = prompt(reader, out, "Git user.name", existing.GitName)
	edited.GitEmail = prompt(reader, out, "Git user.email", existing.GitEmail)

	// Provider: updated only via --provider flag (Q3 / D-09). No interactive
	// prompt — the provider marker is persisted (D-11) and the flag is the
	// explicit opt-in to overwrite it. Without the flag the existing provider
	// (and its SSH Host block marker) is preserved on rewrite.
	if flags.provider != "" {
		if err := identity.ValidateProvider(flags.provider); err != nil {
			return fmt.Errorf("identity update: %w", err)
		}
		edited.Provider = flags.provider
	}

	edited.Alias = prompt(reader, out, "Host alias", existing.Alias)
	edited.Hostname = prompt(reader, out, "Hostname", existing.Hostname)
	edited.Port = promptPort(reader, out, "Port", existing.Port)

	// Match strategy: flag-or-picker (D-07, D-09, D-10). Pre-fill the picker from
	// existing.Matches so a hasconfig identity does NOT silently collapse to
	// gitdir (Pitfall 6 regression guard).
	gitdirDefault := "~/git/" + name + "/"
	urlDefault := defaultURLPattern(existing.Hostname, name)
	switch {
	case flags.gitdir != "" && flags.url != "":
		edited.Matches = buildMatches("3", flags.gitdir, flags.url)
	case flags.gitdir != "":
		edited.Matches = buildMatches("1", flags.gitdir, "")
	case flags.url != "":
		edited.Matches = buildMatches("2", "", flags.url)
	case flags.match != "":
		// --match flag: non-interactive parity surface (D-10). Use current values
		// as defaults for the strategy-dependent sub-fields so a strategy change
		// without explicit --gitdir/--url stays in the same directory/URL.
		currentGitdir := gitdirDefault
		currentURL := urlDefault
		for _, m := range existing.Matches {
			switch m.Kind {
			case gitconfig.MatchGitdir:
				currentGitdir = m.Value
			case gitconfig.MatchHasconfig:
				currentURL = strings.TrimPrefix(m.Value, "remote.*.url:")
			}
		}
		stratNum, ferr := matchFromFlag(flags.match)
		if ferr != nil {
			return ferr
		}
		edited.Matches = buildMatches(stratNum, currentGitdir, currentURL)
	default:
		// Extract current gitdir/url values for pre-fill.
		currentGitdir := gitdirDefault
		currentURL := urlDefault
		for _, m := range existing.Matches {
			switch m.Kind {
			case gitconfig.MatchGitdir:
				currentGitdir = m.Value
			case gitconfig.MatchHasconfig:
				// Strip "remote.*.url:" prefix to get the bare URL pattern for the prompt.
				currentURL = strings.TrimPrefix(m.Value, "remote.*.url:")
			}
		}
		edited.Matches = promptMatchStrategy(reader, out, currentGitdir, currentURL, strategyNumFromKind(matchKinds(existing.Matches)))
	}

	// Signing on/off toggle.
	signingLabel := "n"
	if currentSigning {
		signingLabel = "y"
	}
	signingAnswer := strings.ToLower(strings.TrimSpace(prompt(reader, out, "Enable commit signing? [y/n]", signingLabel)))
	signing := signingAnswer == "y" || signingAnswer == "yes"

	// Preview the update.
	fp(out, "\n=== Preview: updated identity ===\n")
	fp(out, fmt.Sprintf("  name:     %s  (unchanged)\n", name))
	fp(out, fmt.Sprintf("  git:      %s <%s>\n", edited.GitName, edited.GitEmail))
	fp(out, fmt.Sprintf("  alias:    %s\n", edited.Alias))
	fp(out, fmt.Sprintf("  hostname: %s\n", edited.Hostname))
	fp(out, fmt.Sprintf("  port:     %d\n", edited.Port))
	fp(out, fmt.Sprintf("  match:    %s\n", renderMatches(edited.Matches)))
	if signing {
		fp(out, "  signing:  on\n")
	} else {
		fp(out, "  signing:  off\n")
	}

	if dryRun {
		fp(out, "\n--dry-run: no files were written.\n")
		return nil
	}

	// D-16: check for overlapping match conditions before updating.
	// Build the "other" identities list: all accounts except the one being updated.
	var others []identity.Account
	for _, a := range accounts {
		if a.Name != name {
			others = append(others, a)
		}
	}
	prospective := identity.Account{
		Name:    edited.Name,
		Matches: edited.Matches,
	}
	if !warnOverlapAndConfirm(reader, out, prospective, others) {
		fp(out, "Update cancelled; no files were written.\n")
		return nil
	}

	if !confirm(reader, out, "Apply these changes now?") {
		fp(out, "Update cancelled; no files were written.\n")
		return nil
	}

	deps := depsFor(out)
	res, err := identity.Update(existing, edited, deps, signing)
	if err != nil {
		return fmt.Errorf("identity update: %w", err)
	}

	fp(out, "\nIdentity updated.\n")

	if res.Structural {
		fp(out, "\nResolved test (structural change detected):\n")
		fp(out, fmt.Sprintf("$ %s\n%s\n", res.ResolvedTest.Command, strings.TrimRight(res.ResolvedTest.Output, "\n")))
		if res.Resolved.User != "" {
			fp(out, fmt.Sprintf("  user=%s hostname=%s port=%s identitiesonly=%s\n",
				res.Resolved.User, res.Resolved.Hostname, res.Resolved.Port, res.Resolved.IdentitiesOnly))
		}
	}

	return nil
}

// buildUpdateDeps wires identity.UpdateDeps from the real internal packages,
// following the same pattern as buildDeps in add.go.
func buildUpdateDeps(_ io.Writer) identity.UpdateDeps {
	return identity.UpdateDeps{
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
			// Ensure the global allowed_signers pointer is set (idempotent).
			if serr := gitconfig.SetAllowedSignersFile(gitconfigPath, allowedSignersPath); serr != nil {
				return backup, serr
			}
			return backup, nil
		},
		WriteFragment: func(fragPath, name, email, signingKeyPath string, signing bool) error {
			return gitconfig.WriteFragment(fragPath, name, email, signingKeyPath, signing)
		},
		WriteAllowedSigners: keygen.WriteAllowedSigners,
		RemoveAllowedSigners: func(path, name string) (string, error) {
			return gitconfig.RemoveAllowedSignersBlock(path, name)
		},
		Resolved: tester.Resolved,
		ReadPub: func(pubPath string) (string, error) {
			data, rerr := os.ReadFile(pubPath) //nolint:gosec // gitid-managed .pub path (G304)
			if rerr != nil {
				return "", rerr
			}
			return strings.TrimRight(string(data), "\n"), nil
		},
	}
}
