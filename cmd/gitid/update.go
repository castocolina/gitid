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

// newUpdateCmd builds `gitid identity update <name>` (IDENT-04). The handler is
// thin: it validates the name, loads the current identity via reconstruction,
// prompts for edits with pre-filled current values, previews, confirms, and calls
// identity.Update. All orchestration logic lives in internal/identity.Update.
func newUpdateCmd() *cobra.Command {
	var dryRun bool
	cmd := &cobra.Command{
		Use:   "update <name>",
		Short: "Update an existing Git identity's fields (email, signing, alias, port, match strategy — name immutable)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runIdentityUpdate(cmd.InOrStdin(), cmd.OutOrStdout(), args[0], dryRun, buildUpdateDeps)
		},
	}
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "preview the update without writing anything (SAFE-03)")
	return cmd
}

// runIdentityUpdate is the update orchestration handler. It validates name,
// reconstructs the current identity from disk, prompts for edits pre-filled with
// current values, previews, confirms (unless --dry-run), and calls identity.Update.
func runIdentityUpdate(in io.Reader, out io.Writer, name string, dryRun bool, depsFor func(io.Writer) identity.UpdateDeps) error {
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

	// Provider is NOT prompted: loader.Reconstruct DERIVES Account.Provider from
	// the alias (TrimPrefix(alias, name+".") with a hostname fallback) — it is
	// never persisted as an independent artifact, and Update writes no provider
	// field. A standalone Provider edit therefore could not round-trip, so the
	// prompt is omitted to avoid implying an edit that cannot persist (finding
	// #4). The alias prompt below is the real lever for changing the provider.
	edited.Alias = prompt(reader, out, "Host alias", existing.Alias)
	edited.Hostname = prompt(reader, out, "Hostname", existing.Hostname)
	edited.Port = promptPort(reader, out, "Port", existing.Port)

	matchDefault := ""
	if len(existing.Matches) > 0 {
		matchDefault = renderMatches(existing.Matches)
	} else {
		matchDefault = "~/git/" + name + "/"
	}
	newMatchDir := prompt(reader, out, "Match gitdir", matchDefault)
	edited.Matches = []gitconfig.Match{{Kind: gitconfig.MatchGitdir, Value: newMatchDir}}

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
	fp(out, fmt.Sprintf("  match:    %s\n", newMatchDir))
	if signing {
		fp(out, "  signing:  on\n")
	} else {
		fp(out, "  signing:  off\n")
	}

	if dryRun {
		fp(out, "\n--dry-run: no files were written.\n")
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
