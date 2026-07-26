package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/spf13/cobra"

	"github.com/castocolina/gitid/internal/gitconfig"
	"github.com/castocolina/gitid/internal/identity"
)

// sanitizeName trims surrounding whitespace from a user-supplied identity name
// before validation.
func sanitizeName(name string) string {
	return strings.TrimSpace(name)
}

// identityNameRe is the allowed charset for a gitid identity name passed to
// rotate: letters, digits, dot, underscore, and hyphen. It rejects whitespace
// and shell/newline metacharacters so the name can never inject into an
// arg-slice exec or break a managed block (T-02-32).
var identityNameRe = regexp.MustCompile(`^[A-Za-z0-9._-]+$`)

// newRotateCmd builds `gitid identity rotate <name>` (KEY-01). The handler is
// thin: it validates the name, reconstructs the account's gitid-managed paths,
// confirms (SAFE-03), calls identity.Rotate, and prints the re-test output. All
// re-point orchestration lives in internal/identity.Rotate.
func newRotateCmd() *cobra.Command {
	var dryRun bool
	cmd := &cobra.Command{
		Use:   "rotate <name>",
		Short: "Rotate (replace) the key for an existing identity, re-pointing all four artifacts",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runIdentityRotate(cmd.InOrStdin(), cmd.OutOrStdout(), args[0], dryRun, buildDeps)
		},
	}
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "preview the rotation without writing anything (SAFE-03)")
	return cmd
}

// runIdentityRotate validates name, gathers the existing identity's account
// (provider/alias/git details and the gitid-managed target paths), asks one
// explicit confirmation (skipped under --dry-run), calls identity.Rotate, and
// prints the pre-write + resolved two-phase test output (KEY-01 re-test).
func runIdentityRotate(in io.Reader, out io.Writer, name string, dryRun bool, depsFor func(io.Writer) identity.Deps) error {
	name = sanitizeName(name)
	if name == "" {
		return fmt.Errorf("identity rotate: identity name is required")
	}
	if !identityNameRe.MatchString(name) {
		return fmt.Errorf("identity rotate: invalid identity name %q (allowed: letters, digits, '.', '_', '-')", name)
	}

	reader := bufio.NewReader(in)
	account, err := gatherRotateAccount(reader, out, name)
	if err != nil {
		return err
	}

	if !dryRun {
		if !confirm(reader, out, fmt.Sprintf("Rotate the key for %q and re-point all four artifacts now?", name)) {
			fp(out, "Rotation cancelled; no files were written.\n")
			return nil
		}
	}

	deps := depsFor(out)
	res, err := identity.Rotate(account, deps)
	if err != nil {
		return err
	}

	printPreWrite(out, res.PreWrite)
	printPreview(out, res)

	if dryRun {
		fp(out, "\n--dry-run: no files were rotated.\n")
		return nil
	}

	if !res.PreWriteOnly {
		loadKeyIntoAgent(out, res.Key.PrivatePath)
		printResolved(out, res)
	}
	fp(out, "\n"+uploadInstructions(account.Provider)+"\n")
	printPubForManualCopy(out, res.Key.PubLine)
	return nil
}

// gatherRotateAccount reconstructs the Account to rotate. gitid does not yet
// persist a load layer, so the existing identity's details are gathered via
// prompts with sensible defaults and the gitid-managed paths resolved from HOME.
// The key path follows the D-06 convention id_<algo>_<name>.
func gatherRotateAccount(r *bufio.Reader, out io.Writer, name string) (identity.Account, error) {
	gitName := prompt(r, out, "Git user.name", "")
	gitEmail := prompt(r, out, "Git user.email", "")
	provider := prompt(r, out, "Provider (github/gitlab)", "github")
	alias := prompt(r, out, "Host alias", identity.DefaultAlias(name, provider))
	hostname := prompt(r, out, "Hostname", defaultHostname(provider))
	port := promptPort(r, out, "Port", 443)
	matchDir := prompt(r, out, "Match gitdir", "~/git/"+name+"/")

	home, err := os.UserHomeDir()
	if err != nil {
		return identity.Account{}, fmt.Errorf("identity rotate: resolving home dir: %w", err)
	}

	keyPath := filepath.Join(home, ".ssh", "id_ed25519_"+name)
	return identity.Account{
		Name:               name,
		GitName:            gitName,
		GitEmail:           gitEmail,
		Provider:           provider,
		Alias:              alias,
		Hostname:           hostname,
		Port:               port,
		KeyPath:            keyPath,
		PubPath:            keyPath + ".pub",
		Matches:            []gitconfig.Match{{Kind: gitconfig.MatchGitdir, Value: matchDir}},
		FragmentPath:       filepath.Join(home, ".gitconfig.d", name),
		GitconfigPath:      filepath.Join(home, ".gitconfig"),
		SSHConfigPath:      filepath.Join(home, ".ssh", "config"),
		AllowedSignersPath: filepath.Join(home, ".ssh", "allowed_signers"),
	}, nil
}
