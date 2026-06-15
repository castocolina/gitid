package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/castocolina/gitid/internal/clipboard"
	"github.com/castocolina/gitid/internal/gitconfig"
	"github.com/castocolina/gitid/internal/identity"
	"github.com/castocolina/gitid/internal/upload"
)

// newCopyCmd builds `gitid copy <name>` (CLI-01 / D-06). Copies the public key
// for the named identity to the clipboard and prints provider upload instructions.
func newCopyCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "copy <name>",
		Short: "Copy the public key to the clipboard and print upload instructions",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runCopy(cmd.OutOrStdout(), args[0])
		},
	}
}

// newIdentityCopyCmd builds `gitid identity copy <name>` (CLI-01 / D-06).
// Same behavior as the top-level copy command.
func newIdentityCopyCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "copy <name>",
		Short: "Copy the public key to the clipboard and print upload instructions",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runCopy(cmd.OutOrStdout(), args[0])
		},
	}
}

// newHostAddCmd builds `gitid host add` (CLI-01 / D-07). Adds a host alias
// (SSH account) to an existing identity, delegating to the add-account flow.
func newHostAddCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "add",
		Short: "Add a host alias (SSH account) to an existing identity",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runAddAccount(bufio.NewReader(cmd.InOrStdin()), cmd.OutOrStdout(), false, buildDeps)
		},
	}
}

// runCopy is the handler for `gitid copy <name>` and `gitid identity copy <name>`.
// It validates the identity name (T-05-05), reconstructs the account from managed
// config files, reads the public key (.pub), copies it to the clipboard
// (CLIP-02 graceful degradation on no-tool), and prints provider upload
// instructions via internal/upload.Instructions (UP-02).
func runCopy(out io.Writer, name string) error {
	// T-05-05: validate name before any filesystem access.
	name = sanitizeName(name)
	if err := identity.ValidateName(name); err != nil {
		return fmt.Errorf("copy: %w", err)
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("copy: resolving home dir: %w", err)
	}

	// Read managed config files to reconstruct account list.
	sshBytes, _ := os.ReadFile(filepath.Join(home, ".ssh", "config")) //nolint:gosec // trusted gitid-managed path (G304)
	gcBytes, _ := os.ReadFile(filepath.Join(home, ".gitconfig"))      //nolint:gosec // trusted gitid-managed path (G304)

	accounts, err := identity.Reconstruct(sshBytes, gcBytes, gitconfig.ReadFragment)
	if err != nil {
		return fmt.Errorf("copy: reconstructing identities: %w", err)
	}

	// Find the account by name.
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
		return fmt.Errorf("copy: identity %q not found", name)
	}

	// Derive the .pub path from the account KeyPath.
	pubPath := acct.PubPath
	if pubPath == "" && acct.KeyPath != "" {
		pubPath = acct.KeyPath + ".pub"
	}
	if pubPath == "" {
		return fmt.Errorf("copy: identity %q has no key path; was it created with gitid?", name)
	}

	pubBytes, err := os.ReadFile(pubPath) //nolint:gosec // trusted gitid-managed path (G304)
	if err != nil {
		return fmt.Errorf("copy: reading public key for %q: %w", name, err)
	}
	pubLine := strings.TrimSpace(string(pubBytes))

	// Copy to clipboard (CLIP-02: on failure, print key for manual copy and continue).
	copyErr := clipboard.Copy(pubLine)
	if copyErr != nil {
		fp(out, fmt.Sprintf("! clipboard copy failed [info]\n%v\nKey is printed below — copy manually.\n\n", copyErr))
	} else {
		fp(out, fmt.Sprintf("Copied public key for %q to clipboard.\n", name))
	}

	// Always print the key line (whether clipboard succeeded or not).
	fp(out, "Key: "+pubLine+"\n\n")

	// Print provider upload instructions (UP-02).
	provider := acct.Provider
	if provider == "" {
		provider = "unknown"
	}
	fp(out, upload.Instructions(provider)+"\n")

	return nil
}
