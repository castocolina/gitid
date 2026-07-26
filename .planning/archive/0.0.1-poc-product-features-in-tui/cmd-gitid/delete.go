package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/castocolina/gitid/internal/filewriter"
	"github.com/castocolina/gitid/internal/gitconfig"
	"github.com/castocolina/gitid/internal/identity"
)

// newDeleteCmd builds `gitid identity delete <name>` (IDENT-05). The handler
// is thin: it validates the name, loads the account via reconstruction, prints
// the "will remove" manifest, confirms (SAFE-03), optionally prompts for key
// deletion (D-07), calls identity.Delete, and prints backup paths.
func newDeleteCmd() *cobra.Command {
	var dryRun bool
	cmd := &cobra.Command{
		Use:   "delete <name>",
		Short: "Delete a gitid-managed identity — removes its four managed artifacts with backup (IDENT-05)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runIdentityDelete(cmd.InOrStdin(), cmd.OutOrStdout(), args[0], dryRun, buildDeleteDeps)
		},
	}
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "preview the removal manifest without writing anything (SAFE-03)")
	return cmd
}

// runIdentityDelete is the delete orchestration handler. It validates name,
// reconstructs the identity from disk, prints the "will remove" manifest,
// confirms (SAFE-03), takes a second explicit prompt for irreversible key
// deletion (D-07, default no), calls identity.Delete, and reports backups.
func runIdentityDelete(in io.Reader, out io.Writer, name string, dryRun bool, depsFor func(io.Writer) identity.DeleteDeps) error {
	name = sanitizeName(name)
	if name == "" {
		return fmt.Errorf("identity delete: identity name is required")
	}
	if !identityNameRe.MatchString(name) {
		return fmt.Errorf("identity delete: invalid identity name %q (allowed: letters, digits, '.', '_', '-')", name)
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("identity delete: resolving home dir: %w", err)
	}

	sshConfigPath := filepath.Join(home, ".ssh", "config")
	gitconfigPath := filepath.Join(home, ".gitconfig")

	sshBytes, err := os.ReadFile(sshConfigPath) //nolint:gosec // gitid-managed path
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("identity delete: reading %s: %w", sshConfigPath, err)
	}
	gcBytes, err := os.ReadFile(gitconfigPath) //nolint:gosec // gitid-managed path
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("identity delete: reading %s: %w", gitconfigPath, err)
	}

	accounts, err := identity.Reconstruct(sshBytes, gcBytes, gitconfig.ReadFragment)
	if err != nil {
		return fmt.Errorf("identity delete: reconstructing identities: %w", err)
	}

	// Find the account matching the requested name.
	var acct identity.Account
	found := false
	for _, a := range accounts {
		if a.Name == name {
			acct = a
			found = true
			break
		}
	}
	if !found {
		return fmt.Errorf("identity delete: no gitid-managed identity named %q (run 'gitid identity list' to see all identities)", name)
	}

	// Fill gitid-managed paths from HOME in case reconstruction left them empty
	// (mirrors the pattern from update.go / rotate.go gatherRotateAccount).
	if acct.FragmentPath == "" {
		acct.FragmentPath = filepath.Join(home, ".gitconfig.d", name)
	}
	if acct.GitconfigPath == "" {
		acct.GitconfigPath = gitconfigPath
	}
	if acct.SSHConfigPath == "" {
		acct.SSHConfigPath = sshConfigPath
	}
	if acct.AllowedSignersPath == "" {
		acct.AllowedSignersPath = filepath.Join(home, ".ssh", "allowed_signers")
	}
	if acct.KeyPath == "" {
		acct.KeyPath = filepath.Join(home, ".ssh", "id_ed25519_"+name)
	}
	if acct.PubPath == "" {
		acct.PubPath = acct.KeyPath + ".pub"
	}

	// Print the "will remove" manifest so the user sees exactly what will be
	// deleted before the single confirmation prompt (D-08).
	fp(out, "Will remove:\n")
	fp(out, fmt.Sprintf("  [1] SSH Host block     %q in %s\n", "# BEGIN gitid managed: "+name, acct.SSHConfigPath))
	fp(out, fmt.Sprintf("  [2] gitconfig block    %q in %s\n", "# BEGIN gitid managed: "+name, acct.GitconfigPath))
	fp(out, fmt.Sprintf("  [3] Fragment file      %s\n", acct.FragmentPath))
	if acct.GitEmail != "" {
		fp(out, fmt.Sprintf("  [4] allowed_signers    line for <%s> in %s\n", acct.GitEmail, acct.AllowedSignersPath))
	}

	if dryRun {
		fp(out, "\n--dry-run: no files were written.\n")
		return nil
	}

	reader := bufio.NewReader(in)

	// First confirm: the user must explicitly consent to remove the managed
	// blocks and fragment file (SAFE-03). Block/file removals are reversible
	// (backups exist); key deletion is gated behind a separate prompt.
	if !confirm(reader, out, "Remove these managed blocks and the fragment file now?") {
		fp(out, "Delete cancelled; no files were written.\n")
		return nil
	}

	// Second prompt: key deletion is irreversible — D-07 requires a separate
	// explicit confirmation defaulting to "no". Pressing Enter (or typing 'n')
	// keeps the key safe.
	keepKey := !confirm(reader, out, "Also delete the private key files? (irreversible, default no)")

	deps := depsFor(out)
	res, err := identity.Delete(acct, keepKey, deps)
	if err != nil {
		return fmt.Errorf("identity delete: %w", err)
	}

	// Print backup paths for the user's reference.
	fp(out, "\nIdentity deleted.\n")
	if res.SSHBackup != "" {
		fp(out, fmt.Sprintf("  SSH config backup:      %s\n", res.SSHBackup))
	}
	if res.GitconfigBackup != "" {
		fp(out, fmt.Sprintf("  gitconfig backup:       %s\n", res.GitconfigBackup))
	}
	if res.FragmentBackup != "" {
		fp(out, fmt.Sprintf("  fragment file backup:   %s\n", res.FragmentBackup))
	}
	if res.AllowedSignersBackup != "" {
		fp(out, fmt.Sprintf("  allowed_signers backup: %s\n", res.AllowedSignersBackup))
	}
	if keepKey {
		fp(out, "  Private key files:      kept (not deleted)\n")
	} else {
		fp(out, "  Private key files:      removed (backed up)\n")
		if res.KeyBackup != "" {
			fp(out, fmt.Sprintf("  private key backup:     %s\n", res.KeyBackup))
		}
		if res.PubBackup != "" {
			fp(out, fmt.Sprintf("  public key backup:      %s\n", res.PubBackup))
		}
	}

	return nil
}

// buildDeleteDeps wires identity.DeleteDeps from the real internal packages,
// following the same pattern as buildDeps in add.go.
func buildDeleteDeps(_ io.Writer) identity.DeleteDeps {
	home, _ := os.UserHomeDir()
	sshConfigPath := filepath.Join(home, ".ssh", "config")
	gitconfigPath := filepath.Join(home, ".gitconfig")

	return identity.DeleteDeps{
		ReadSSH: func() ([]byte, error) {
			data, err := os.ReadFile(sshConfigPath) //nolint:gosec // gitid-managed path
			if os.IsNotExist(err) {
				return []byte{}, nil
			}
			return data, err
		},
		ReadGitconfig: func() ([]byte, error) {
			data, err := os.ReadFile(gitconfigPath) //nolint:gosec // gitid-managed path
			if os.IsNotExist(err) {
				return []byte{}, nil
			}
			return data, err
		},
		WriteSSH: func(content []byte) (string, error) {
			return filewriter.Write(sshConfigPath, content, 0o600)
		},
		WriteGitconfig: func(content []byte) (string, error) {
			return filewriter.Write(gitconfigPath, content, 0o600)
		},
		RemoveFragment: filewriter.BackupAndRemove,
		RemoveAllowedSigners: func(path, name string) (string, error) {
			return gitconfig.RemoveAllowedSignersBlock(path, name)
		},
		// Route key removal through filewriter.BackupAndRemove so the private
		// material is preserved as a timestamped .bak.<ts> (mode 0600) and the
		// removal is atomic per file (CR-02). A missing file is a no-op.
		RemoveKeyFiles: func(keyPath, pubPath string) (string, string, error) {
			keyBak, kerr := filewriter.BackupAndRemove(keyPath)
			if kerr != nil {
				return "", "", fmt.Errorf("removing private key %s: %w", keyPath, kerr)
			}
			pubBak, perr := filewriter.BackupAndRemove(pubPath)
			if perr != nil {
				return keyBak, "", fmt.Errorf("removing public key %s: %w", pubPath, perr)
			}
			return keyBak, pubBak, nil
		},
	}
}
