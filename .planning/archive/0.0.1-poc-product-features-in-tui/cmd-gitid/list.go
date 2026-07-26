package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/castocolina/gitid/internal/gitconfig"
	"github.com/castocolina/gitid/internal/identity"
)

// newListCmd builds `gitid identity list` (IDENT-03). The handler is thin:
// it reads ~/.ssh/config and ~/.gitconfig, calls identity.Reconstruct, and
// renders each account's key path, alias, provider, port, and match strategy,
// plus a light incomplete marker for partial block sets (D-02/D-03).
func newListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List gitid-managed identities reconstructed from ~/.ssh/config and ~/.gitconfig (IDENT-03)",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runIdentityList(cmd.InOrStdin(), cmd.OutOrStdout())
		},
	}
	return cmd
}

// runIdentityList is the list orchestration handler. It resolves HOME,
// reads the two managed config files (missing file = empty, not an error —
// T-03-07), calls identity.Reconstruct with the real gitconfig.ReadFragment,
// and renders each Account grouped by identity (D-03).
func runIdentityList(_ io.Reader, out io.Writer) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("identity list: resolving home dir: %w", err)
	}

	sshConfigPath := filepath.Join(home, ".ssh", "config")
	gitconfigPath := filepath.Join(home, ".gitconfig")

	sshBytes, err := os.ReadFile(sshConfigPath) //nolint:gosec // sshConfigPath is a gitid-managed path
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("identity list: reading %s: %w", sshConfigPath, err)
	}

	gcBytes, err := os.ReadFile(gitconfigPath) //nolint:gosec // gitconfigPath is a gitid-managed path
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("identity list: reading %s: %w", gitconfigPath, err)
	}

	accounts, err := identity.Reconstruct(sshBytes, gcBytes, gitconfig.ReadFragment)
	if err != nil {
		return fmt.Errorf("identity list: reconstructing identities: %w", err)
	}

	if len(accounts) == 0 {
		fp(out, "no gitid-managed identities found\n")
		return nil
	}

	printAccounts(out, accounts)
	return nil
}

// printAccounts renders the reconstructed accounts grouped by identity (D-03).
// For each account it prints: identity header (name, key path, git name/email),
// then alias, provider, port, and match strategy per account.
// When acct.Incomplete is non-empty it appends a light marker line (D-02).
func printAccounts(out io.Writer, accounts []identity.Account) {
	for i, acct := range accounts {
		if i > 0 {
			fp(out, "\n")
		}
		// Identity header: name + key path + git name/email.
		fp(out, fmt.Sprintf("identity: %s\n", acct.Name))
		if acct.KeyPath != "" {
			fp(out, fmt.Sprintf("  key:      %s\n", acct.KeyPath))
		}
		if acct.GitName != "" || acct.GitEmail != "" {
			fp(out, fmt.Sprintf("  git:      %s <%s>\n", acct.GitName, acct.GitEmail))
		}

		// Per-account fields: alias, provider, port, match strategy.
		if acct.Alias != "" {
			fp(out, fmt.Sprintf("  alias:    %s\n", acct.Alias))
		}
		provider := acct.Provider
		if provider == "" && acct.Hostname != "" {
			// A1: when provider cannot be derived from the alias, fall back to Hostname.
			provider = acct.Hostname
		}
		if provider != "" {
			fp(out, fmt.Sprintf("  provider: %s\n", provider))
		}
		if acct.Port != 0 {
			fp(out, fmt.Sprintf("  port:     %d\n", acct.Port))
		}
		if len(acct.Matches) > 0 {
			fp(out, fmt.Sprintf("  match:    %s\n", renderMatches(acct.Matches)))
		}

		// Light incompleteness marker (D-02): name what is missing, never diagnose.
		if acct.Incomplete != "" {
			fp(out, fmt.Sprintf("  ! incomplete: missing %s\n", acct.Incomplete))
		}
	}
}

// renderMatches formats the match strategy for the list view. Multiple matches
// are joined by "; ". Each match uses its condition string for display.
func renderMatches(matches []gitconfig.Match) string {
	parts := make([]string, 0, len(matches))
	for _, m := range matches {
		switch m.Kind {
		case gitconfig.MatchGitdir:
			v := m.Value
			if !strings.HasSuffix(v, "/") {
				v += "/"
			}
			parts = append(parts, "gitdir:"+v)
		case gitconfig.MatchHasconfig:
			parts = append(parts, "hasconfig:"+m.Value)
		default:
			parts = append(parts, m.Value)
		}
	}
	return strings.Join(parts, "; ")
}
