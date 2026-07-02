package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/castocolina/gitid/internal/adopter"
	"github.com/castocolina/gitid/internal/filewriter"
	"github.com/castocolina/gitid/internal/gitconfig"
	"github.com/castocolina/gitid/internal/identity"
)

// newAdoptCmd builds `gitid adopt <fragment-path>` (ADOPT-01).
// Adopts a plain-style gitconfig fragment into gitid management either by
// migrating (copy) or referencing in-place, then writing an [includeIf] block.
func newAdoptCmd() *cobra.Command {
	var method string
	var name string
	var yes bool
	cmd := &cobra.Command{
		Use:   "adopt <fragment-path>",
		Short: "Adopt a plain-style gitconfig fragment into gitid management",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runAdopt(cmd.OutOrStdout(), args[0], method, name, yes, buildAdoptDeps)
		},
	}
	cmd.Flags().StringVar(&method, "method", "migrate", "adoption method: migrate or reference")
	cmd.Flags().StringVar(&name, "name", "", "identity name (derived from filename when absent)")
	cmd.Flags().BoolVar(&yes, "yes", false, "skip confirmation prompts (non-interactive)")
	return cmd
}

// runAdopt is the handler for `gitid adopt <fragment-path>`.
// Resolution order for identity name: --name flag > filename suffix > error.
// Migrate never auto-removes the original; the user must delete it manually.
func runAdopt(out io.Writer, fragPath, methodStr, name string, yes bool, depsFor func() (adopter.Deps, error)) error {
	deps, err := depsFor()
	if err != nil {
		return fmt.Errorf("adopt: building deps: %w", err)
	}

	// Resolve identity name: flag → filename suffix → error.
	if name == "" {
		base := filepath.Base(fragPath)
		suffix, ok := strings.CutPrefix(base, ".gitconfig_")
		if ok && suffix != "" {
			name = suffix
		}
	}
	if name == "" {
		return fmt.Errorf("adopt: cannot derive identity name from %q — use --name to specify", fragPath)
	}
	if err := identity.ValidateName(name); err != nil {
		return fmt.Errorf("adopt: %w", err)
	}

	// Resolve method.
	var adoptMethod adopter.AdoptMethod
	switch strings.ToLower(methodStr) {
	case "migrate", "":
		adoptMethod = adopter.AdoptMigrate
	case "reference":
		adoptMethod = adopter.AdoptReferenceInPlace
	default:
		return fmt.Errorf("adopt: unknown method %q — use migrate or reference", methodStr)
	}

	// Resolve gitconfigPath for the backend (deps captures it in the closure).
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("adopt: resolving home dir: %w", err)
	}
	gitconfigPath := filepath.Join(home, ".gitconfig")

	// Build a default gitdir match (~/git/<name>/) per canonical recipe.
	// The adopt command does not currently expose a --match flag; callers
	// can extend this in future plans. The default is consistent with
	// identity.DefaultMatch which uses gitdir:~/git/<name>/.
	defaultMatches := []gitconfig.Match{
		{Kind: gitconfig.MatchGitdir, Value: "~/git/" + name + "/"},
	}

	fp(out, fmt.Sprintf("Adopting %q as identity %q (method: %s)\n", fragPath, name, methodStr))

	result, err := adopter.Adopt(fragPath, name, gitconfigPath, adoptMethod, defaultMatches, deps)
	if err != nil {
		return fmt.Errorf("adopt: %w", err)
	}

	// Print steps.
	if result.MigratedPath != "" {
		fp(out, fmt.Sprintf("  Copied fragment to %s\n", result.MigratedPath))
	}
	for _, bak := range result.BackupPaths {
		fp(out, fmt.Sprintf("  Backup: %s\n", bak))
	}
	fp(out, fmt.Sprintf("  Written includeIf block in %s\n", gitconfigPath))
	fp(out, "Done. Original fragment preserved at its original path.\n")
	if adoptMethod == adopter.AdoptMigrate && !yes {
		fp(out, "Tip: you may remove the original fragment manually after verifying the managed copy.\n")
	}

	return nil
}

// buildAdoptDeps wires adopter.Deps from real internal packages.
// This is the CLI equivalent of buildTUIAdopterDeps() in tui/deps.go.
func buildAdoptDeps() (adopter.Deps, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return adopter.Deps{}, fmt.Errorf("adopt: resolving home dir: %w", err)
	}
	gitconfigPath := filepath.Join(home, ".gitconfig")

	return adopter.Deps{
		ReadFile: func(path string) ([]byte, error) {
			return os.ReadFile(path) //nolint:gosec // trusted gitid-managed path (G304)
		},
		WriteFile: func(path string, content []byte, mode os.FileMode) (string, error) {
			return filewriter.Write(path, content, mode)
		},
		// filewriter.CopyFile returns (backupPath, error); adaptor drops backupPath.
		// The parent directory of dst is created first (filewriter.Write needs it to
		// exist, and the destination is ~/.gitconfig.d/ which may not yet exist).
		CopyFile: func(src, dst string) error {
			if mkErr := filewriter.EnsureDir(filepath.Dir(dst), 0o700); mkErr != nil {
				return fmt.Errorf("adopt: ensuring dest dir: %w", mkErr)
			}
			_, err := filewriter.CopyFile(src, dst)
			return err
		},
		BackupAndRemove: filewriter.BackupAndRemove,
		WriteIncludeIf: func(id, fragPath string, matches []gitconfig.Match) (string, error) {
			return gitconfig.WriteIncludeIf(gitconfigPath, id, fragPath, matches)
		},
		ReadFragment:   gitconfig.ReadFragment,
		ListCandidates: adopter.ListCandidatesFromHome,
	}, nil
}
