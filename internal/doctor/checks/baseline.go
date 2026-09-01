// Package checks implements the per-family health check functions for
// gitid doctor. Each family lives in its own file and is overwritten in
// place by Wave 2 plans without redeclaration.
package checks

import (
	"io"
	"path/filepath"
	"strings"

	"github.com/castocolina/gitid/internal/doctor"
	"github.com/castocolina/gitid/internal/gitconfig"
	"github.com/castocolina/gitid/internal/globalgit"
)

// CheckBaseline checks the four Phase 3.1 baseline invariants (D-16):
//
//  1. core.excludesfile wiring via a six-branch decision tree per GIGN-01.
//  2. Baseline [include] resolves — the managed baseline-include block exists
//     (state.Installed == true).
//  3. core.ignorecase drift — state.BaselineKeys["core.ignorecase"] must equal
//     "false" (locked-value carve-out, D-17).
//  4. Curated gitignore entries — informational when the user-edited managed
//     block omits curated entries (Block is user-owned as of GIGN-01).
//
// The six-branch excludesfile decision tree (Check 2):
//
//	Branch A: key unset                                  → WARNING with pair fix
//	Branch B2: key set, NOT the managed target,  missing  → ERROR, no fix
//	Branch B: key set, NOT the managed target,  exists   → INFO, no fix
//	Branch C: key set,     the managed target,  missing  → ERROR with pair fix
//	Branch D: key set,     the managed target,  exists, block empty/absent → WARNING with pair fix
//	Branch E: key set,     the managed target,  exists, block populated    → (no error)
//
// Dep installs are out of scope. The function only reads via injected
// ReadBaselineState and never writes (D-01).
func CheckBaseline(d doctor.Deps) []doctor.Finding {
	if d.ReadBaselineState == nil {
		return nil
	}

	state, err := d.ReadBaselineState(d.GitconfigPath, d.BaselineFilePath, d.GitignorePath)
	if err != nil {
		// Report the read error as an error finding rather than panicking.
		return []doctor.Finding{{
			Family:      doctor.FamilyBaseline,
			Severity:    doctor.SeverityError,
			Title:       "baseline: could not read state",
			Explanation: err.Error(),
		}}
	}

	var findings []doctor.Finding

	// Check 1: baseline [include] block.
	// If the baseline is not installed (Installed=false) AND not incomplete, the
	// entire baseline has never been set up. Report as one error with fix.
	if !state.Installed {
		// Wire the real fixer: restore the baseline [include] block via
		// deps.AddWiring(GitconfigPath, "baseline-include", "baseline-include:<baselineFilePath>").
		// The AddWiring dispatcher in the cmd layer calls gitconfig.WriteBaselineInclude.
		var fix *doctor.FixDescriptor
		if d.AddWiring != nil && d.GitconfigPath != "" && d.BaselineFilePath != "" {
			gitconfigPath := d.GitconfigPath
			baselineFilePath := d.BaselineFilePath
			addWiring := d.AddWiring
			fix = &doctor.FixDescriptor{
				Summary: "restore baseline [include] block in ~/.gitconfig",
				Fn: func() error {
					return addWiring(gitconfigPath, gitconfig.BaselineIncludeBlockName,
						gitconfig.BaselineIncludeBlockName+":"+baselineFilePath)
				},
			}
		}
		// Fix A: prefer the COMPLETE baseline setup (fragment + gitignore + include,
		// atomically) over the include-only restore above — restoring just the
		// include leaves a dangling pointer to a missing fragment, which can never
		// satisfy this check. When SetupBaseline is wired, run the full
		// `gitid baseline setup` flow interactively (or with defaults under --yes).
		if d.SetupBaseline != nil {
			setup := d.SetupBaseline
			if fix == nil {
				fix = &doctor.FixDescriptor{}
			}
			fix.Summary = "run 'gitid baseline setup' (restores the baseline fragment + include)"
			fix.Interactive = func(in io.Reader, out io.Writer, assumeYes bool) error {
				return setup(in, out, assumeYes)
			}
		}
		findings = append(findings, doctor.Finding{
			Family:       doctor.FamilyBaseline,
			Severity:     doctor.SeverityError,
			Title:        "baseline [include] block missing from ~/.gitconfig",
			Explanation:  "The managed baseline include block is gone. Baseline settings have no effect.",
			SuggestedFix: "run 'gitid baseline setup'",
			Fix:          fix,
		})
		if !blockIsPopulated(state.GitignorePatterns) {
			findings = append(findings, gitignorePairFinding(d, doctor.SeverityWarning,
				"core.excludesfile and global gitignore are not configured",
				"Git has no configured global ignore file. OS/editor artifacts may be committed."))
		}
		return findings
	}

	// Check 2: core.excludesfile and its managed pattern file are one pair.
	// Six-branch decision tree per GIGN-01: the managed block is now user-owned,
	// so the fix must not silently overwrite deliberate user choices.
	// Read from state.BaselineKeys (parsed directly from the baseline
	// fragment's own body by ReadBaselineState) — NEVER via a
	// RunGitConfigGet(d.GitconfigPath, ...) query. `git config --file <path>`
	// does not follow [include] directives when resolving a key (verified
	// empirically: `git config --file ~/.gitconfig core.excludesfile` exits
	// 1 even when the key is set inside the fragment ~/.gitconfig includes)
	// — and gitid's OWN baseline setup always places core.excludesfile
	// inside the included fragment, never directly in ~/.gitconfig. A
	// --file-scoped query would therefore report "not configured" for
	// EVERY correctly-configured baseline, a false positive of exactly the
	// class this whole phase exists to close.
	excludesFile := state.BaselineKeys["core.excludesfile"]
	fileExists := excludesFileExists(d, excludesFile)
	isManagedTarget := pointsToManagedTarget(d, excludesFile)
	blockPopulated := blockIsPopulated(state.GitignorePatterns)

	// Branch A: key unset.
	if excludesFile == "" {
		findings = append(findings, gitignorePairFinding(d, doctor.SeverityWarning,
			"core.excludesfile and global gitignore are not configured",
			"Git has no configured global ignore file. OS/editor artifacts may be committed."))
	} else if !isManagedTarget {
		// Key is set to a DIFFERENT path (not the managed target).
		if !fileExists {
			// Branch B2: wrong target, missing file → ERROR, no fix.
			findings = append(findings, doctor.Finding{
				Family:       doctor.FamilyBaseline,
				Target:       "Git",
				Severity:     doctor.SeverityError,
				Title:        "core.excludesfile points to a missing file: " + excludesFile,
				Explanation:  "Git silently tolerates this dangling excludesfile path. OS/editor artifacts may be committed.",
				SuggestedFix: "create the file, or use the Global Git Ignore screen to adopt the managed target",
				Fix:          nil, // deliberate: cannot silently retarget a user-chosen path
			})
		} else {
			// Branch B: wrong target, existing file → INFO, no fix.
			findings = append(findings, doctor.Finding{
				Family:       doctor.FamilyBaseline,
				Target:       "Git",
				Severity:     doctor.SeverityInfo,
				Title:        "core.excludesfile: " + excludesFile + " (differs from gitid-managed target)",
				Explanation:  "This value is a deliberate user choice. Gitid will not override it.",
				SuggestedFix: "if you want to use the managed global gitignore, change core.excludesfile to the managed target via the Global Git Ignore screen",
				Fix:          nil,
			})
		}
	} else {
		// Key is set to the MANAGED target.
		if !fileExists {
			// Branch C: managed target, missing file → ERROR with fix.
			findings = append(findings, gitignorePairFinding(d, doctor.SeverityError,
				"core.excludesfile points to a missing global gitignore",
				"Git silently tolerates this dangling excludesfile path, so OS/editor artifacts may be committed."))
		} else if !blockPopulated {
			// Branch D: managed target, file exists, block empty/absent → WARNING with fix.
			findings = append(findings, gitignorePairFinding(d, doctor.SeverityWarning,
				"~/.gitignore_global: curated entries missing",
				"Git has a configured gitignore file, but the gitid-managed block contains no patterns."))
		}
		// Branch E: managed target, file exists, block populated → no error.
	}

	findings = append(findings, setDiffersFindings(d)...)

	// Check 4: curated gitignore entries (GIGN-01 user-owned block).
	// The managed block is now editable; this check is informational only,
	// naming what the user removed and how to restore.
	if blockPopulated {
		// Only check when the block has content; Branch D above handles empty blocks.
		existing := make(map[string]bool, len(state.GitignorePatterns))
		for _, p := range state.GitignorePatterns {
			existing[p] = true
		}
		var missing []string
		for _, p := range gitconfig.DefaultGitignoreEntries() {
			if !existing[p] {
				missing = append(missing, p)
			}
		}
		if len(missing) > 0 {
			findings = append(findings, doctor.Finding{
				Family:       doctor.FamilyBaseline,
				Severity:     doctor.SeverityInfo,
				Title:        "~/.gitignore_global: curated entries absent",
				Explanation:  "The managed gitignore block is editable and user-owned. These curated entries are currently absent, which is a deliberate user choice.",
				SuggestedFix: "use the Global Git Ignore screen's Reset action to restore all curated entries",
				Fix:          nil, // informational, not auto-fixable
			})
		}
	}

	return findings
}

func setDiffersFindings(d doctor.Deps) []doctor.Finding {
	if d.RunGitConfigGet == nil || d.GitconfigPath == "" {
		return nil
	}
	var findings []doctor.Finding
	for _, policy := range globalgit.Policy {
		for _, member := range policy.Members {
			value, err := d.RunGitConfigGet(d.GitconfigPath, member.Key)
			if err != nil || value == "" || strings.EqualFold(value, member.Recommended) {
				continue
			}
			findings = append(findings, doctor.Finding{
				Family:      doctor.FamilyBaseline,
				Target:      "Git",
				Severity:    doctor.SeverityInfo,
				Title:       member.Key + ": " + value + " (differs from recommendation)",
				Explanation: "This value is a deliberate user choice. Gitid will not override it.",
			})
		}
	}
	return findings
}

func excludesFileExists(d doctor.Deps, path string) bool {
	if d.Stat == nil || path == "" {
		return false
	}
	_, err := d.Stat(path)
	return err == nil
}

func blockIsPopulated(patterns []string) bool {
	return len(patterns) > 0
}

func pointsToManagedTarget(d doctor.Deps, configuredPath string) bool {
	if configuredPath == "" || d.GitignorePath == "" {
		return false
	}
	// Expand tilde against the directory containing the gitignore file.
	// This works because RenderBaselineBlock writes the literal ~/.gitignore_global
	// and the live fixture already carries that spelling against an absolute
	// GitignorePath. We must not compare strings directly; the bare tilde would
	// never match an absolute path.
	homeDir := filepath.Dir(d.GitignorePath)
	configuredExpanded := configuredPath
	if strings.HasPrefix(configuredPath, "~/") {
		configuredExpanded = filepath.Join(homeDir, configuredPath[2:])
	}
	return configuredExpanded == d.GitignorePath
}

func gitignorePairFinding(d doctor.Deps, severity doctor.Severity, title, explanation string) doctor.Finding {
	var fix *doctor.FixDescriptor
	if d.FixExcludesfile != nil && d.GitignorePath != "" {
		path := d.GitignorePath
		fixExcludesfile := d.FixExcludesfile
		fix = &doctor.FixDescriptor{
			Summary: "configure core.excludesfile and the global gitignore",
			Fn: func() error {
				return fixExcludesfile(path)
			},
		}
	}
	return doctor.Finding{
		Family:       doctor.FamilyBaseline,
		Target:       "Git",
		Severity:     severity,
		Title:        title,
		Explanation:  explanation,
		SuggestedFix: "configure the managed global gitignore",
		Fix:          fix,
	}
}
