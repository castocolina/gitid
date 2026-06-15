package gitconfig

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/castocolina/gitid/internal/filewriter"
)

// BaselineConfig holds the Tier-2 (optional) fields for the baseline block.
// Tier-1 keys (ignorecase, excludesfile, push.autoSetupRemote, pull.rebase,
// fetch.prune, color.ui) are unconditional and never gated.
type BaselineConfig struct {
	// AutoCRLF controls whether core.autocrlf = input is written (Tier-2).
	AutoCRLF bool
	// Pager is the value for core.pager (Tier-2). Empty string omits the key.
	Pager string
	// ExtraColors controls whether color.branch/diff/status=auto are written.
	ExtraColors bool
	// DiffColorMoved controls whether diff.colorMoved=zebra is written (Tier-2).
	DiffColorMoved bool
	// MergeConflictStyle is the value for merge.conflictstyle (e.g. "zdiff3").
	// Empty string omits the [merge] section entirely (C4 git-version gate).
	MergeConflictStyle string
	// InitDefaultBranch is the value for init.defaultBranch (Tier-2).
	// Empty string omits the [init] section.
	InitDefaultBranch string
	// IncludeAliases controls whether the [alias] section is written (Tier-2).
	IncludeAliases bool
}

// DefaultBaselineConfig returns a BaselineConfig with all Tier-2 options
// enabled and set to the values from the gist reference (D-04).
func DefaultBaselineConfig() BaselineConfig {
	return BaselineConfig{
		AutoCRLF:           true,
		Pager:              "less -FRX",
		ExtraColors:        true,
		DiffColorMoved:     true,
		MergeConflictStyle: "zdiff3",
		InitDefaultBranch:  "main",
		IncludeAliases:     true,
	}
}

// URLRewrite is one HTTPS→SSH insteadOf mapping. HTTPSPrefix is the HTTPS URL
// prefix (e.g. "https://github.com/") and SSHPrefix is the replacement SSH
// target (e.g. "git@github.com:").
type URLRewrite struct {
	HTTPSPrefix string
	SSHPrefix   string
}

// DefaultURLRewrites returns the three big-three HTTPS→SSH mappings (D-05/D-06).
// Order is fixed: github.com, gitlab.com, bitbucket.org (determinism, Pitfall D).
func DefaultURLRewrites() []URLRewrite {
	return []URLRewrite{
		{HTTPSPrefix: "https://github.com/", SSHPrefix: "git@github.com:"},
		{HTTPSPrefix: "https://gitlab.com/", SSHPrefix: "git@gitlab.com:"},
		{HTTPSPrefix: "https://bitbucket.org/", SSHPrefix: "git@bitbucket.org:"},
	}
}

// DefaultGitignorePatterns returns the curated gitignore seed list (D-08/SC-2).
// The first six are SC-2-locked; the remaining are planner discretion (D-Claude).
// Order is fixed to satisfy the byte-stability contract (Pitfall D).
func DefaultGitignorePatterns() []string {
	return []string{
		".DS_Store",
		"Thumbs.db",
		"*.log",
		"*.bak",
		"*.tmp",
		"*.swp",
		"*.swo",
		".idea/",
		".vscode/",
		"node_modules/",
		"__pycache__/",
		"*.pyc",
		".env",
	}
}

// RenderBaselineBlock renders the baseline gitconfig block body with fixed
// section ordering ([core],[push],[pull],[fetch],[color],[diff],[merge],[init],
// [alias]) and tab-prefixed keys — matching the SC-1 idempotency contract and
// RESEARCH Example 1. Calling it twice with the same cfg yields identical bytes.
//
// Tier-1 keys (ignorecase, excludesfile, push.autoSetupRemote, pull.rebase,
// fetch.prune, color.ui) are always written. Tier-2 keys are gated on cfg
// fields. No user section is ever emitted (D-04b). core.editor is never
// seeded (D-12).
//
// Any user-supplied string in cfg (Pager, MergeConflictStyle, InitDefaultBranch)
// is validated with validateValue before render.
func RenderBaselineBlock(cfg BaselineConfig) string {
	// Validate user-supplied Tier-2 strings before rendering (V5 injection guard).
	if cfg.Pager != "" {
		if err := validateValue("core.pager", cfg.Pager); err != nil {
			panic(fmt.Sprintf("gitconfig: RenderBaselineBlock: %v", err))
		}
	}
	if cfg.MergeConflictStyle != "" {
		if err := validateValue("merge.conflictstyle", cfg.MergeConflictStyle); err != nil {
			panic(fmt.Sprintf("gitconfig: RenderBaselineBlock: %v", err))
		}
	}
	if cfg.InitDefaultBranch != "" {
		if err := validateValue("init.defaultBranch", cfg.InitDefaultBranch); err != nil {
			panic(fmt.Sprintf("gitconfig: RenderBaselineBlock: %v", err))
		}
	}

	var b strings.Builder

	// [core] — Tier-1 keys always first, Tier-2 (autocrlf, pager) conditional.
	fmt.Fprintf(&b, "[core]\n")
	fmt.Fprintf(&b, "\tignorecase = false\n")
	fmt.Fprintf(&b, "\texcludesfile = ~/.gitignore_global\n")
	if cfg.AutoCRLF {
		fmt.Fprintf(&b, "\tautocrlf = input\n")
	}
	if cfg.Pager != "" {
		fmt.Fprintf(&b, "\tpager = %s\n", cfg.Pager)
	}

	// [push] — Tier-1
	fmt.Fprintf(&b, "[push]\n")
	fmt.Fprintf(&b, "\tautoSetupRemote = true\n")

	// [pull] — Tier-1
	fmt.Fprintf(&b, "[pull]\n")
	fmt.Fprintf(&b, "\trebase = true\n")

	// [fetch] — Tier-1
	fmt.Fprintf(&b, "[fetch]\n")
	fmt.Fprintf(&b, "\tprune = true\n")

	// [color] — ui is Tier-1; branch/diff/status are Tier-2
	fmt.Fprintf(&b, "[color]\n")
	fmt.Fprintf(&b, "\tui = auto\n")
	if cfg.ExtraColors {
		fmt.Fprintf(&b, "\tbranch = auto\n")
		fmt.Fprintf(&b, "\tdiff = auto\n")
		fmt.Fprintf(&b, "\tstatus = auto\n")
	}

	// [diff] — Tier-2
	if cfg.DiffColorMoved {
		fmt.Fprintf(&b, "[diff]\n")
		fmt.Fprintf(&b, "\tcolorMoved = zebra\n")
	}

	// [merge] — Tier-2; omit entirely when MergeConflictStyle is empty (C4 gate)
	if cfg.MergeConflictStyle != "" {
		fmt.Fprintf(&b, "[merge]\n")
		fmt.Fprintf(&b, "\tconflictstyle = %s\n", cfg.MergeConflictStyle)
	}

	// [init] — Tier-2
	if cfg.InitDefaultBranch != "" {
		fmt.Fprintf(&b, "[init]\n")
		fmt.Fprintf(&b, "\tdefaultBranch = %s\n", cfg.InitDefaultBranch)
	}

	// [alias] — Tier-2; fixed alias order from D-04a / gist reference.
	if cfg.IncludeAliases {
		fmt.Fprintf(&b, "[alias]\n")
		fmt.Fprintf(&b, "\tst = status\n")
		fmt.Fprintf(&b, "\tco = checkout\n")
		fmt.Fprintf(&b, "\tbr = branch\n")
		fmt.Fprintf(&b, "\tci = commit\n")
		fmt.Fprintf(&b, "\tdf = diff\n")
		fmt.Fprintf(&b, "\tlg = log --graph --pretty=format:'%%Cred%%h%%Creset -%%C(yellow)%%d%%Creset %%s %%Cgreen(%%cr) %%C(bold blue)<%%an>%%Creset' --abbrev-commit\n")
		fmt.Fprintf(&b, "\tunstage = reset HEAD --\n")
		fmt.Fprintf(&b, "\tlast = log -1 HEAD\n")
	}

	return strings.TrimRight(b.String(), "\n")
}

// RenderURLRewritesBlock renders the url-rewrites block body for the given
// insteadOf mappings. The section order matches the input slice order (callers
// use DefaultURLRewrites for the canonical big-three order). Each URL/SSH prefix
// pair is validated with validateValue before render to guard against newline
// injection. An empty rewrites slice returns an empty string.
func RenderURLRewritesBlock(rewrites []URLRewrite) string {
	if len(rewrites) == 0 {
		return ""
	}

	var b strings.Builder
	for _, r := range rewrites {
		// Validate user-supplied URL strings (V5 injection guard).
		if err := validateValue("url.insteadOf.httpsPrefix", r.HTTPSPrefix); err != nil {
			panic(fmt.Sprintf("gitconfig: RenderURLRewritesBlock: %v", err))
		}
		if err := validateValue("url.insteadOf.sshPrefix", r.SSHPrefix); err != nil {
			panic(fmt.Sprintf("gitconfig: RenderURLRewritesBlock: %v", err))
		}
		fmt.Fprintf(&b, "[url %q]\n", r.SSHPrefix)
		fmt.Fprintf(&b, "\tinsteadOf = %s\n", r.HTTPSPrefix)
	}
	return strings.TrimRight(b.String(), "\n")
}

// RenderGitignoreBlock renders the gitignore block body — one pattern per line
// in the fixed order of the patterns slice. An empty slice returns an empty
// string. The order must match DefaultGitignorePatterns for SC-2 compliance
// and byte-stability (Pitfall D).
func RenderGitignoreBlock(patterns []string) string {
	if len(patterns) == 0 {
		return ""
	}
	var b strings.Builder
	for _, p := range patterns {
		fmt.Fprintf(&b, "%s\n", p)
	}
	return strings.TrimRight(b.String(), "\n")
}

// WriteBaselineFile composes the baseline and url-rewrites managed blocks into
// baselineFilePath through the filewriter chokepoint. It creates the file's
// parent directory (mode 0700) if it does not already exist, consistent with
// WriteFragment's EnsureDir pattern. The baseline block is always written; the
// url-rewrites block is written when len(rewrites) > 0 and removed otherwise
// (D-07 independent toggling). Foreign content outside either block is preserved
// verbatim (D-02). It returns the backup path (empty when the file is new).
func WriteBaselineFile(baselineFilePath string, cfg BaselineConfig, rewrites []URLRewrite) (string, error) {
	if err := filewriter.EnsureDir(filepath.Dir(baselineFilePath), 0o700); err != nil {
		return "", fmt.Errorf("ensuring baseline dir: %w", err)
	}

	existing, err := os.ReadFile(baselineFilePath) //nolint:gosec // baselineFilePath is a trusted gitid-managed path
	if err != nil && !os.IsNotExist(err) {
		return "", fmt.Errorf("reading %s: %w", baselineFilePath, err)
	}

	composed := filewriter.ReplaceBlock(existing, "baseline", RenderBaselineBlock(cfg))
	if len(rewrites) > 0 {
		composed = filewriter.ReplaceBlock(composed, "url-rewrites", RenderURLRewritesBlock(rewrites))
	} else {
		composed = filewriter.RemoveBlock(composed, "url-rewrites")
	}

	backupPath, err := filewriter.Write(baselineFilePath, composed, gitconfigMode)
	if err != nil {
		return "", fmt.Errorf("writing baseline block to %s: %w", baselineFilePath, err)
	}
	return backupPath, nil
}

// WriteGlobalGitignore composes the gitignore managed block into gitignorePath
// through the filewriter chokepoint. Foreign content outside the managed block
// is preserved verbatim (D-09). It returns the backup path (empty when the file
// is new).
func WriteGlobalGitignore(gitignorePath string, patterns []string) (string, error) {
	existing, err := os.ReadFile(gitignorePath) //nolint:gosec // gitignorePath is a trusted gitid-managed path
	if err != nil && !os.IsNotExist(err) {
		return "", fmt.Errorf("reading %s: %w", gitignorePath, err)
	}

	composed := filewriter.ReplaceBlock(existing, "gitignore", RenderGitignoreBlock(patterns))
	backupPath, err := filewriter.Write(gitignorePath, composed, gitconfigMode)
	if err != nil {
		return "", fmt.Errorf("writing gitignore block to %s: %w", gitignorePath, err)
	}
	return backupPath, nil
}

// WriteBaselineInclude prepends a managed [include] block pointing at
// baselineFilePath into gitconfigPath, placing the block at the TOP of the
// file (floor model — D-10, RESEARCH C1). The include path is written as a
// fixed literal string with a literal ~ so git expands it at runtime (RESEARCH
// Open Q2). The sentinel name is "baseline-include" (distinct from "baseline").
//
// On first write the block is prepended before all existing content. On
// subsequent writes (re-runs) the block is updated in-place via ReplaceBlock so
// its floor position is preserved. It returns the backup path (empty when the
// file is new).
func WriteBaselineInclude(gitconfigPath, baselineFilePath string) (string, error) {
	// The include body is a fixed constant; baselineFilePath is an in-process
	// constant (never user input), so T-03.1-03 is satisfied — no user string
	// is interpolated into the include body.
	_ = baselineFilePath // used as documentation; literal path written below
	const includeBody = "[include]\n\tpath = ~/.gitconfig.d/00-baseline"

	existing, err := os.ReadFile(gitconfigPath) //nolint:gosec // gitconfigPath is a trusted gitid-managed path
	if err != nil && !os.IsNotExist(err) {
		return "", fmt.Errorf("reading %s: %w", gitconfigPath, err)
	}

	composed := filewriter.PrependBlockIfNotFound(existing, "baseline-include", includeBody)
	backupPath, err := filewriter.Write(gitconfigPath, composed, gitconfigMode)
	if err != nil {
		return "", fmt.Errorf("writing baseline-include block to %s: %w", gitconfigPath, err)
	}
	return backupPath, nil
}
