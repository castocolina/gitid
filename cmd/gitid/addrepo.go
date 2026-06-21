package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/castocolina/gitid/internal/repoclone"
	"github.com/castocolina/gitid/internal/sshconfig"
)

// newAddRepoCmd builds `gitid add repo <url>` (REPO-01).
// Detects the provider from the URL, rewrites to the SSH alias, clones, and pulls.
func newAddRepoCmd() *cobra.Command {
	var client string
	var yes bool
	cmd := &cobra.Command{
		Use:   "repo <url>",
		Short: "Clone a repository using the matching gitid identity",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runAddRepo(cmd.OutOrStdout(), args[0], client, yes, buildRepoCloneDeps)
		},
	}
	cmd.Flags().StringVar(&client, "client", "", "client/project folder name under ~/git/")
	cmd.Flags().BoolVar(&yes, "yes", false, "skip confirmation prompts (non-interactive)")
	return cmd
}

// runAddRepo is the handler for `gitid add repo <url>`.
// It detects the provider, looks up the SSH alias, rewrites the URL, clones, and pulls.
// Clone and pull output are ALWAYS printed (PRD §4.7 / REVIEWS.md #2).
func runAddRepo(out io.Writer, rawURL, client string, _ bool, depsFor func() repoclone.Deps) error {
	deps := depsFor()

	// Look up a managed identity whose SSH alias resolves to this provider.
	home, err := deps.UserHomeDir()
	if err != nil {
		return fmt.Errorf("add repo: resolving home dir: %w", err)
	}
	sshConfigPath := filepath.Join(home, ".ssh", "config")
	sshBytes, _ := os.ReadFile(sshConfigPath) //nolint:gosec // trusted gitid-managed path (G304)
	managedHosts, _ := sshconfig.ParseManagedHosts(sshBytes)

	// Attempt provider detection; local (file://) and custom URLs may have no provider.
	effectiveURL := rawURL
	provider, provErr := repoclone.ProviderFromURL(rawURL)
	if provErr == nil {
		// Find alias whose Hostname or Provider matches the detected provider.
		alias := findAliasForProvider(managedHosts, provider)

		// Rewrite the URL to use the SSH alias (when one exists), otherwise use as-is.
		if alias != "" {
			rewritten, rwErr := repoclone.RewriteToAlias(rawURL, alias)
			if rwErr == nil {
				effectiveURL = rewritten
				fp(out, fmt.Sprintf("Rewriting URL: %s\n  -> %s\n", rawURL, effectiveURL))
			}
		} else {
			fp(out, fmt.Sprintf("No managed identity found for provider %q — cloning with original URL.\n", provider))
		}
	}

	// Derive destination path.
	baseDir := filepath.Join(home, "git")
	destPath, err := repoclone.DestPath(baseDir, client, rawURL)
	if err != nil {
		// Fall back: use the last path segment of the URL (handles file:// and
		// non-standard URLs). Strip trailing slash and .git suffix.
		repoName := filepath.Base(strings.TrimSuffix(strings.TrimRight(rawURL, "/"), ".git"))
		if repoName == "" || repoName == "." || repoName == "/" {
			return fmt.Errorf("add repo: computing dest path: %w", err)
		}
		destPath = filepath.Join(baseDir, client, repoName)
	}
	fp(out, fmt.Sprintf("Cloning into %s\n", destPath))

	// Clone — print output always.
	cloneLines, cloneErr := repoclone.Clone(effectiveURL, destPath, deps)
	for _, l := range cloneLines {
		if strings.TrimSpace(l) != "" {
			fp(out, "  "+l+"\n")
		}
	}
	if cloneErr != nil {
		return fmt.Errorf("add repo: clone: %w", cloneErr)
	}

	// Pull — print output always.
	fp(out, "Running git pull...\n")
	pullLines, pullErr := repoclone.Pull(destPath, deps)
	for _, l := range pullLines {
		if strings.TrimSpace(l) != "" {
			fp(out, "  "+l+"\n")
		}
	}
	if pullErr != nil {
		// Non-fatal: a fresh clone that just succeeded may return a trivial pull message.
		fp(out, fmt.Sprintf("  (pull: %v)\n", pullErr))
	}

	fp(out, fmt.Sprintf("Done. Repository cloned at %s\n", destPath))
	return nil
}

// findAliasForProvider scans the managed SSH host map for an entry whose
// Provider marker or Hostname matches provider. Returns "" when no match found.
//
// Match order:
//  1. Exact provider marker match (info.Provider == provider, case-insensitive).
//  2. Suffix/contains match: provider hostname is a suffix of info.Provider or
//     info.Hostname (e.g. URL "github.com" matches SSH block with Provider "github"
//     or Hostname "github.com" / "ssh.github.com"). Covers both the .com-qualified
//     URL hostname and the short provider names stored by identity add.
func findAliasForProvider(hosts map[string]sshconfig.SSHHostInfo, provider string) string {
	provLower := strings.ToLower(provider)

	// First pass: exact provider marker match.
	for _, info := range hosts {
		if strings.ToLower(info.Provider) == provLower {
			return info.Alias
		}
	}

	// Second pass: provider is a suffix of or equal to the stored provider/hostname.
	// Handles the common case where the URL gives "github.com" but the stored
	// provider marker is "github" (short name), or Hostname is "ssh.github.com".
	for _, info := range hosts {
		infoProvLower := strings.ToLower(info.Provider)
		infoHostLower := strings.ToLower(info.Hostname)
		// Short name "github" is contained in "github.com".
		if infoProvLower != "" && strings.Contains(provLower, infoProvLower) {
			return info.Alias
		}
		// Hostname "ssh.github.com" or "github.com" ends with provider "github.com".
		if strings.HasSuffix(infoHostLower, provLower) {
			return info.Alias
		}
	}
	return ""
}

// buildRepoCloneDeps wires repoclone.Deps from the real internal packages.
// This is the CLI equivalent of buildTUIRepoCloneDeps() in tui/deps.go.
// All exec lives in repoclone (liveClone / livePull) — addrepo.go never calls os/exec.
func buildRepoCloneDeps() repoclone.Deps {
	return repoclone.Deps{
		Stat:        os.Stat,
		Clone:       repoclone.LiveClone,
		Pull:        repoclone.LivePull,
		UserHomeDir: os.UserHomeDir,
	}
}
