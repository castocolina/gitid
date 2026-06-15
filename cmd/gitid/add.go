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
	"github.com/castocolina/gitid/internal/gitconfig"
	"github.com/castocolina/gitid/internal/identity"
	"github.com/castocolina/gitid/internal/keygen"
	"github.com/castocolina/gitid/internal/platform"
	"github.com/castocolina/gitid/internal/sshconfig"
	"github.com/castocolina/gitid/internal/tester"
)

// newAddCmd builds `gitid identity add` (create-new mode). The handler is thin:
// it gathers input, builds identity.Deps from the real internal packages, calls
// identity.Create, and prints. All orchestration logic lives in
// internal/identity.Create (no business logic in cmd/).
func newAddCmd() *cobra.Command {
	var dryRun bool
	cmd := &cobra.Command{
		Use:   "add",
		Short: "Create a new Git identity (key, SSH config, gitconfig, allowed_signers)",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runIdentityAdd(cmd.InOrStdin(), cmd.OutOrStdout(), dryRun, buildDeps)
		},
	}
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "preview the four artifacts without writing anything (SAFE-03)")
	return cmd
}

// runIdentityAdd is the create-new orchestration handler. It probes the
// algorithm (D-14 stop on none), gathers inputs via prompts (D-05), builds the
// four-writer Deps wiring keygen.WriteAllowedSigners and
// gitconfig.SetAllowedSignersFile, calls identity.Create, prints the test
// command+output (TEST-03) and the unified four-artifact preview, asks one
// explicit confirmation (skipped under --dry-run, SAFE-03), and on confirm loads
// the key into the agent (ssh-add, D-08) and prints upload steps.
func runIdentityAdd(in io.Reader, out io.Writer, dryRun bool, depsFor func(io.Writer) identity.Deps) error {
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
	input, err := gatherCreateInput(reader, out, algo, dryRun)
	if err != nil {
		return err
	}

	deps := depsFor(out)
	res, err := identity.Create(input, deps)
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

// fp writes s to out, ignoring the write error: out is the command's stdout,
// where a write failure is neither recoverable nor actionable.
func fp(out io.Writer, s string) {
	_, _ = io.WriteString(out, s)
}

// gatherCreateInput collects the create-new inputs via interactive prompts with
// sensible defaults shown (D-05). The host alias is pre-selected to
// <identity>.<provider> (D-12) and the match defaults to gitdir:~/git/<id>/
// (D-13). Confirmed is the single explicit y/N consent, skipped under dryRun.
func gatherCreateInput(r *bufio.Reader, out io.Writer, algo string, dryRun bool) (identity.CreateInput, error) {
	name := prompt(r, out, "Identity name", "")
	if name == "" {
		return identity.CreateInput{}, fmt.Errorf("identity add: identity name is required")
	}
	gitName := prompt(r, out, "Git user.name", "")
	gitEmail := prompt(r, out, "Git user.email", "")
	provider := prompt(r, out, "Provider (github/gitlab)", "github")
	alias := prompt(r, out, "Host alias", identity.DefaultAlias(name, provider))
	hostname := prompt(r, out, "Hostname", defaultHostname(provider))
	port := promptPort(r, out, "Port", 443)
	matchDir := prompt(r, out, "Match gitdir", "~/git/"+name+"/")
	passphrase := prompt(r, out, "Passphrase (empty for none)", "")

	home, err := os.UserHomeDir()
	if err != nil {
		return identity.CreateInput{}, fmt.Errorf("identity add: resolving home dir: %w", err)
	}

	in := identity.CreateInput{
		Name:               name,
		GitName:            gitName,
		GitEmail:           gitEmail,
		Provider:           provider,
		Algo:               algo,
		Alias:              alias,
		Hostname:           hostname,
		Port:               port,
		Passphrase:         passphrase,
		Matches:            []gitconfig.Match{{Kind: gitconfig.MatchGitdir, Value: matchDir}},
		FragmentPath:       filepath.Join(home, ".gitconfig.d", name),
		GitconfigPath:      filepath.Join(home, ".gitconfig"),
		SSHConfigPath:      filepath.Join(home, ".ssh", "config"),
		AllowedSignersPath: filepath.Join(home, ".ssh", "allowed_signers"),
		GlobalBlock:        sshconfig.RenderGlobalBlock(platform.CurrentOS()),
	}

	if dryRun {
		in.Confirmed = false
		return in, nil
	}
	in.Confirmed = confirm(r, out, "Write all four artifacts now?")
	return in, nil
}

// buildDeps wires identity.Deps from the real internal packages, including the
// FOURTH writer keygen.WriteAllowedSigners and the global pointer
// gitconfig.SetAllowedSignersFile inside the gitconfig writer.
func buildDeps(_ io.Writer) identity.Deps {
	return identity.Deps{
		Generate: func(in identity.CreateInput) (identity.KeyResult, error) {
			home, herr := os.UserHomeDir()
			if herr != nil {
				return identity.KeyResult{}, herr
			}
			r, gerr := keygen.Generate(keygen.Params{
				Algo:       in.Algo,
				Identity:   in.Name,
				Comment:    in.Name + "@gitid",
				Passphrase: in.Passphrase,
				Dir:        filepath.Join(home, ".ssh"),
			})
			if gerr != nil {
				return identity.KeyResult{}, gerr
			}
			return identity.KeyResult{PrivatePath: r.PrivatePath, PubPath: r.PubPath, PubLine: r.PubLine}, nil
		},
		CopyPub: clipboard.Copy,
		PreWrite: func(keyPath, host string) tester.Result {
			return tester.PreWrite(keyPath, host)
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
		WriteFragment:       gitconfig.WriteFragment,
		WriteAllowedSigners: keygen.WriteAllowedSigners,
		Resolved:            tester.Resolved,
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
