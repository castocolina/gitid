package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/castocolina/gitid/internal/deps"
	"github.com/castocolina/gitid/internal/gitconfig"
)

// newBaselineSetupCmd builds `gitid baseline setup`. The handler is thin:
// it calls runBaselineSetup with the cobra command's stdin/stdout and the
// --dry-run flag.
func newBaselineSetupCmd() *cobra.Command {
	var dryRun bool
	cmd := &cobra.Command{
		Use:   "setup",
		Short: "Seed the global baseline git config (toggles, aliases, URL rewrites, gitignore)",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runBaselineSetup(cmd.InOrStdin(), cmd.OutOrStdout(), dryRun)
		},
	}
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "preview changes without writing anything (SAFE-03)")
	return cmd
}

// newBaselineShowCmd builds `gitid baseline show`. The handler calls
// runBaselineShow with the cobra command's stdin/stdout.
func newBaselineShowCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "show",
		Short: "Show the current managed baseline state from disk",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runBaselineShow(cmd.InOrStdin(), cmd.OutOrStdout())
		},
	}
	return cmd
}

// runBaselineSetup is the setup orchestration handler (UI-SPEC §"Full Interaction
// Flow"). It: (1) resolves home and builds paths; (2) builds default selections and
// applies the zdiff3 git-version gate; (3) scans for conflicts; (4) prints the
// unified preview; (5) prompts for Tier-2 and rewrite selections; (6) short-circuits
// under --dry-run; (7) confirms once; (8) writes the three surfaces; (9) prints the
// write summary.
func runBaselineSetup(in io.Reader, out io.Writer, dryRun bool) error {
	// Step 1: resolve home and build absolute paths.
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("baseline setup: resolving home dir: %w", err)
	}
	absGitconfig := filepath.Join(home, ".gitconfig")
	absBaseline := filepath.Join(home, ".gitconfig.d", "00-baseline")
	absGitignore := filepath.Join(home, ".gitignore_global")

	// Step 2: build default selections.
	cfg := gitconfig.DefaultBaselineConfig()
	rewrites := gitconfig.DefaultURLRewrites()
	patterns := gitconfig.DefaultGitignorePatterns()

	// C4 zdiff3 git-version gate: omit merge.conflictstyle if git < 2.35.
	if !deps.GitVersionAtLeast(2, 35) {
		cfg.MergeConflictStyle = ""
	}

	// Step 3: scan for conflicts in the user's ~/.gitconfig.
	conflicts, err := gitconfig.ScanConflicts(absGitconfig, gitconfig.BaselineKeySet())
	if err != nil {
		return fmt.Errorf("baseline setup: scanning conflicts: %w", err)
	}

	// Step 4: print the unified preview (UI-SPEC §"Preview Layout Contract").
	if err := printBaselinePreview(out, cfg, rewrites, patterns, conflicts); err != nil {
		return fmt.Errorf("baseline setup: printing preview: %w", err)
	}

	// Step 5: interactive editing (skip under --dry-run).
	reader := bufio.NewReader(in)
	if !dryRun {
		// Tier-2 opt-out prompt (default Y).
		if !promptYN(reader, out, "Include Tier-2 defaults? (autocrlf, pager, branch/diff/status color, zdiff3, main branch, aliases)") {
			cfg.AutoCRLF = false
			cfg.Pager = ""
			cfg.ExtraColors = false
			cfg.DiffColorMoved = false
			cfg.MergeConflictStyle = ""
			cfg.InitDefaultBranch = ""
			cfg.IncludeAliases = false
		}

		// Per-rewrite deselect prompts (default Y).
		var kept []gitconfig.URLRewrite
		for _, r := range rewrites {
			host := extractHost(r.HTTPSPrefix)
			label := fmt.Sprintf("Keep rewrite for %s (%s → %s)?", host, r.HTTPSPrefix, r.SSHPrefix)
			if promptYN(reader, out, label) {
				kept = append(kept, r)
			}
		}
		rewrites = kept
	}

	// Step 6: --dry-run short-circuit.
	if dryRun {
		fp(out, "--dry-run: no files were written.\n")
		return nil
	}

	// Step 7: single confirm gate (SAFE-03, default N per UI-SPEC).
	if !confirm(reader, out, "Write baseline now?") {
		fp(out, "Baseline setup cancelled; no files were written.\n")
		return nil
	}

	// Step 8: write the three managed surfaces with rollback on partial failure
	// (CR-01). The activation [include] line is written LAST so a failure on any
	// earlier surface leaves git's state unchanged (the baseline file is present
	// but not yet wired into ~/.gitconfig — the idempotent repair path handles this).
	// If the baseline file write succeeds but the [include] write fails, the
	// gitignore and baseline file changes are rolled back and the user receives a
	// clear partial-failure error with the backup paths for manual recovery.

	gitignoreBackup, err := gitconfig.WriteGlobalGitignore(absGitignore, patterns)
	if err != nil {
		return fmt.Errorf("baseline setup: writing %s: %w", absGitignore, err)
	}

	baselineBackup, err := gitconfig.WriteBaselineFile(absBaseline, cfg, rewrites)
	if err != nil {
		// Roll back the already-written gitignore before surfacing the error.
		if rollbackErr := restoreBackup(absGitignore, gitignoreBackup); rollbackErr != nil {
			return fmt.Errorf(
				"baseline setup: writing %s failed (%v) AND rollback of %s failed: %w",
				absBaseline, err, absGitignore, rollbackErr)
		}
		return fmt.Errorf("baseline setup: writing %s failed (gitignore rolled back): %w", absBaseline, err)
	}

	gitconfigBackup, err := gitconfig.WriteBaselineInclude(absGitconfig, absBaseline)
	if err != nil {
		// Roll back both prior surfaces before surfacing the error.
		var rollbackErrors []string
		if rerr := restoreBackup(absGitignore, gitignoreBackup); rerr != nil {
			rollbackErrors = append(rollbackErrors, tildePath(absGitignore)+": "+rerr.Error())
		}
		if rerr := restoreBackup(absBaseline, baselineBackup); rerr != nil {
			rollbackErrors = append(rollbackErrors, tildePath(absBaseline)+": "+rerr.Error())
		}
		if len(rollbackErrors) > 0 {
			return fmt.Errorf(
				"baseline setup: writing %s failed (%v) AND rollback failed: %s",
				absGitconfig, err, strings.Join(rollbackErrors, "; "))
		}
		return fmt.Errorf("baseline setup: writing %s failed (gitignore + baseline rolled back): %w", absGitconfig, err)
	}

	// Step 9: print write summary (UI-SPEC §"Write Summary Contract").
	fp(out, "Baseline written.\n")
	printBaselineWriteSummary(out, absGitconfig, gitconfigBackup, absBaseline, baselineBackup, absGitignore, gitignoreBackup)
	return nil
}

// restoreBackup restores a file from its backup path. When backupPath is empty,
// the file did not exist before the write — remove the newly-created file to
// restore the original absent state. Returns nil if the file did not exist and
// backupPath is empty (no-op on already-clean state).
func restoreBackup(targetPath, backupPath string) error {
	if backupPath == "" {
		// File was new (no prior backup) — remove it to undo the write.
		if err := os.Remove(targetPath); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("removing newly-created %s during rollback: %w", targetPath, err)
		}
		return nil
	}
	// File existed before — copy backup back over the target.
	backupData, err := os.ReadFile(backupPath) //nolint:gosec // backupPath is a gitid-managed backup path
	if err != nil {
		return fmt.Errorf("reading backup %s for rollback: %w", backupPath, err)
	}
	if err := os.WriteFile(targetPath, backupData, 0o644); err != nil { //nolint:gosec // targetPath is a gitid-managed path
		return fmt.Errorf("restoring %s from backup during rollback: %w", targetPath, err)
	}
	return nil
}

// runBaselineShow is the show orchestration handler. It resolves home, builds
// the three absolute paths, calls ReadBaselineState, and prints per the
// UI-SPEC §"gitid baseline show — Read-Back Contract". No prompts; no === wrapper.
func runBaselineShow(_ io.Reader, out io.Writer) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("baseline show: resolving home dir: %w", err)
	}
	absGitconfig := filepath.Join(home, ".gitconfig")
	absBaseline := filepath.Join(home, ".gitconfig.d", "00-baseline")
	absGitignore := filepath.Join(home, ".gitignore_global")

	state, err := gitconfig.ReadBaselineState(absGitconfig, absBaseline, absGitignore)
	if err != nil {
		return fmt.Errorf("baseline show: reading state: %w", err)
	}

	printBaselineState(out, state, absGitconfig, absBaseline, absGitignore)
	return nil
}

// printBaselinePreview prints the unified preview (UI-SPEC §"Preview Layout
// Contract") for setup. It is called before interactive editing and the confirm.
// Returns an error when a renderer validation fails (WR-03: no panics on invalid input).
func printBaselinePreview(out io.Writer, cfg gitconfig.BaselineConfig, rewrites []gitconfig.URLRewrite, patterns []string, conflicts []gitconfig.Conflict) error {
	fp(out, "\n=== Preview: baseline setup ===\n")

	// Sub-section 1: Baseline git-config block.
	fp(out, "--- ~/.gitconfig.d/00-baseline (baseline block) ---\n")
	baselineBlock, err := gitconfig.RenderBaselineBlock(cfg)
	if err != nil {
		return fmt.Errorf("rendering baseline block for preview: %w", err)
	}
	fp(out, baselineBlock+"\n")

	// Sub-section 2: URL rewrites block + blast-radius warning.
	fp(out, "--- ~/.gitconfig.d/00-baseline (url-rewrites block) ---\n")
	rewritesBlock, err := gitconfig.RenderURLRewritesBlock(rewrites)
	if err != nil {
		return fmt.Errorf("rendering url-rewrites block for preview: %w", err)
	}
	fp(out, rewritesBlock+"\n")
	fp(out, "  ! insteadOf rewrites affect ALL HTTPS operations for each host —\n")
	fp(out, "  ! including go get, npm install, cargo fetch, and CI pipelines\n")
	fp(out, "  ! using token-based HTTPS auth. SSH agent must be running.\n")

	// Sub-section 3: Global gitignore block.
	fp(out, "--- ~/.gitignore_global (gitignore block) ---\n")
	fp(out, gitconfig.RenderGitignoreBlock(patterns)+"\n")

	// Sub-section 4: include wiring + floor-model note.
	fp(out, "--- ~/.gitconfig (include block, prepended) ---\n")
	fp(out, "# BEGIN gitid managed: baseline-include\n")
	fp(out, "[include]\n")
	fp(out, "\tpath = ~/.gitconfig.d/00-baseline\n")
	fp(out, "# END gitid managed: baseline-include\n")
	fp(out, "  > This block will be placed at the TOP of ~/.gitconfig.\n")
	fp(out, "  > Keys you set elsewhere in ~/.gitconfig override the baseline (floor model).\n")

	// Sub-section 5: Conflict warnings (conditional).
	printConflictSection(out, conflicts)
	return nil
}

// printConflictSection prints the conflict sub-section (UI-SPEC §"Sub-section 5").
func printConflictSection(out io.Writer, conflicts []gitconfig.Conflict) {
	if len(conflicts) == 0 {
		fp(out, "  > No conflicts found in ~/.gitconfig.\n")
		return
	}

	fp(out, "--- Conflicts detected in ~/.gitconfig ---\n")
	excludesfileConflict := false
	for _, c := range conflicts {
		fp(out, fmt.Sprintf("  ! %s: your value=%s  baseline=%s  winner=yours (floor ordering)\n",
			c.Key, c.UserValue, c.BaselineValue))
		if c.Key == "core.excludesfile" {
			excludesfileConflict = true
		}
	}
	if excludesfileConflict {
		fp(out, "  ! Note: your excludesfile override means gitid's ~/.gitignore_global will be\n")
		fp(out, "  ! ignored by git. Remove your override or update it to ~/.gitignore_global.\n")
	}
}

// printBaselineWriteSummary prints the write summary per UI-SPEC §"Write Summary
// Contract". A non-empty backupPath indicates the file existed before (updated
// with backup). An empty backupPath indicates a new file was written.
func printBaselineWriteSummary(out io.Writer, gitconfigPath, gitconfigBackup, baselinePath, baselineBackup, gitignorePath, gitignoreBackup string) {
	fp(out, fmt.Sprintf("  %s backup:          %s\n", tildePath(gitconfigPath), formatBackup(gitconfigBackup)))
	fp(out, fmt.Sprintf("  %s:   %s\n", tildePath(baselinePath), formatWriteStatus(baselineBackup)))
	fp(out, fmt.Sprintf("  %s:          %s\n", tildePath(gitignorePath), formatWriteStatus(gitignoreBackup)))
}

// printBaselineState renders the read-back output per UI-SPEC §"gitid baseline
// show — Read-Back Contract". No === wrapper (intentional per the format note).
func printBaselineState(out io.Writer, state gitconfig.BaselineState, gitconfigPath, baselinePath, gitignorePath string) {
	if !state.Installed && !state.Incomplete {
		fp(out, "no gitid-managed baseline found\n")
		fp(out, "Run 'gitid baseline setup' to initialize.\n")
		return
	}

	if state.Incomplete {
		fp(out, "baseline: incomplete\n")
		for _, m := range state.Missing {
			fp(out, fmt.Sprintf("  ! missing: %s\n", m))
		}
		fp(out, "  Run 'gitid baseline setup' to repair.\n")
		return
	}

	// Installed state.
	fp(out, "baseline: installed\n")
	fp(out, fmt.Sprintf("  file:     %s\n", tildePath(baselinePath)))
	fp(out, fmt.Sprintf("  include:  %s (prepended)\n", tildePath(gitconfigPath)))
	fp(out, "\n")

	// Baseline keys section.
	fp(out, "baseline keys:\n")
	printBaselineKeys(out, state.BaselineKeys)

	// URL rewrites section.
	fp(out, "\n")
	fp(out, fmt.Sprintf("url rewrites: %d active\n", len(state.URLRewrites)))
	for _, r := range state.URLRewrites {
		fp(out, fmt.Sprintf("  %-24s → %s\n", r.HTTPSPrefix, r.SSHPrefix))
	}

	// Gitignore section.
	fp(out, "\n")
	fp(out, fmt.Sprintf("gitignore: %s\n", tildePath(gitignorePath)))
	fp(out, fmt.Sprintf("  managed patterns: %d\n", len(state.GitignorePatterns)))
	if len(state.GitignorePatterns) > 0 {
		fp(out, "  "+strings.Join(state.GitignorePatterns, ", ")+"\n")
	}
}

// printBaselineKeys prints the baseline key-value pairs in the fixed section
// order matching the renderer (UI-SPEC §"Output when baseline is installed").
func printBaselineKeys(out io.Writer, keys map[string]string) {
	// Fixed print order matching RenderBaselineBlock section order.
	orderedKeys := []string{
		"core.ignorecase",
		"core.excludesfile",
		"core.autocrlf",
		"core.pager",
		"push.autosetupremote",
		"pull.rebase",
		"fetch.prune",
		"color.ui",
		"color.branch",
		"color.diff",
		"color.status",
		"diff.colormoved",
		"merge.conflictstyle",
		"init.defaultbranch",
		"alias.st",
		"alias.co",
		"alias.br",
		"alias.ci",
		"alias.df",
		"alias.lg",
		"alias.unstage",
		"alias.last",
	}
	// Display key names (camelCase matching UI-SPEC).
	displayKeys := map[string]string{
		"core.ignorecase":      "core.ignorecase",
		"core.excludesfile":    "core.excludesfile",
		"core.autocrlf":        "core.autocrlf",
		"core.pager":           "core.pager",
		"push.autosetupremote": "push.autoSetupRemote",
		"pull.rebase":          "pull.rebase",
		"fetch.prune":          "fetch.prune",
		"color.ui":             "color.ui",
		"color.branch":         "color.branch",
		"color.diff":           "color.diff",
		"color.status":         "color.status",
		"diff.colormoved":      "diff.colorMoved",
		"merge.conflictstyle":  "merge.conflictstyle",
		"init.defaultbranch":   "init.defaultBranch",
		"alias.st":             "alias.st",
		"alias.co":             "alias.co",
		"alias.br":             "alias.br",
		"alias.ci":             "alias.ci",
		"alias.df":             "alias.df",
		"alias.lg":             "alias.lg",
		"alias.unstage":        "alias.unstage",
		"alias.last":           "alias.last",
	}

	for _, k := range orderedKeys {
		v, ok := keys[k]
		if !ok {
			continue
		}
		display := displayKeys[k]
		fp(out, fmt.Sprintf("  %-22s = %s\n", display, v))
	}
}

// promptYN reads a Y/n prompt (uppercase Y = default YES per D-04/D-06 opt-out
// model). Returns true when user accepts the default or types y/yes.
//
// On a non-EOF read error (e.g. broken pipe, closed stdin) promptYN returns
// false (safe direction) rather than silently treating the error as acceptance
// of an opt-out default that mutates ~/.gitconfig (WR-06).
func promptYN(r *bufio.Reader, out io.Writer, label string) bool {
	fp(out, fmt.Sprintf("%s [Y/n]: ", label))
	line, err := r.ReadString('\n')
	if err != nil && err != io.EOF {
		// Non-EOF read error — fail safe: do not accept opt-out defaults (WR-06).
		return false
	}
	line = strings.ToLower(strings.TrimSpace(line))
	// Default is Y: empty input or clean EOF → accept.
	return line == "" || line == "y" || line == "yes"
}

// extractHost derives the display hostname from an HTTPS URL prefix.
// "https://github.com/" → "github.com".
func extractHost(httpsPrefix string) string {
	s := strings.TrimPrefix(httpsPrefix, "https://")
	s = strings.TrimSuffix(s, "/")
	return s
}

// tildePath replaces the home-directory prefix with ~.
func tildePath(path string) string {
	home, err := os.UserHomeDir()
	if err != nil {
		return path
	}
	if strings.HasPrefix(path, home) {
		return "~" + path[len(home):]
	}
	return path
}

// formatBackup formats the backup path for write summary output.
func formatBackup(backupPath string) string {
	if backupPath == "" {
		return "(new)"
	}
	return tildePath(backupPath)
}

// formatWriteStatus formats the write status for a file in the write summary.
func formatWriteStatus(backupPath string) string {
	if backupPath == "" {
		return "written (new)"
	}
	return fmt.Sprintf("updated (backup: %s)", tildePath(backupPath))
}
