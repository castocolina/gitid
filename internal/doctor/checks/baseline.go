// Package checks implements the per-family health check functions for
// gitid doctor. Each family lives in its own file and is overwritten in
// place by Wave 2 plans without redeclaration.
package checks

import (
	"io"
	"strings"

	"github.com/castocolina/gitid/internal/doctor"
	"github.com/castocolina/gitid/internal/gitconfig"
	"github.com/castocolina/gitid/internal/globalgit"
)

// CheckBaseline checks the four Phase 3.1 baseline invariants (D-16):
//
//  1. core.excludesfile wiring — the key is set in the baseline block.
//  2. Baseline [include] resolves — the managed baseline-include block exists
//     (state.Installed == true).
//  3. core.ignorecase drift — state.BaselineKeys["core.ignorecase"] must equal
//     "false" (locked-value carve-out, D-17).
//  4. Curated excludes present — all DefaultGitignorePatterns are in
//     state.GitignorePatterns.
//
// Severity mapping:
//
//	excludesfile not wired → error (broken: OS artifacts not excluded)
//	include block missing  → error + Fix descriptor (auto-fixable re-add, D-02)
//	ignorecase drift       → warning (degraded, D-17)
//	curated entries absent → warning + Fix descriptor (auto-fixable restore, D-02)
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
		if !matchesDefaultGitignorePatterns(state.GitignorePatterns) {
			findings = append(findings, gitignorePairFinding(d, doctor.SeverityWarning,
				"core.excludesfile and global gitignore are not configured",
				"Git has no configured global ignore file. OS/editor artifacts may be committed."))
		}
		return findings
	}

	// Check 2: core.excludesfile and its managed pattern file are one pair.
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
	gitignorePresent := matchesDefaultGitignorePatterns(state.GitignorePatterns)
	fileExists := excludesFileExists(d, excludesFile)
	if excludesFile == "" {
		if !gitignorePresent {
			findings = append(findings, gitignorePairFinding(d, doctor.SeverityWarning,
				"core.excludesfile and global gitignore are not configured",
				"Git has no configured global ignore file. OS/editor artifacts may be committed."))
		}
	} else if !fileExists {
		// A dangling pointer — the key is wired but the file it names does
		// not exist at all. Content-incompleteness of an EXISTING file is a
		// separate, lower-severity concern (Check 4's "curated entries
		// missing" WARNING below) — conflating the two under this ERROR's
		// "missing" wording would misreport an existing-but-incomplete file
		// as absent, a false positive of exactly the class this phase
		// exists to close.
		findings = append(findings, gitignorePairFinding(d, doctor.SeverityError,
			"core.excludesfile points to a missing global gitignore",
			"Git silently tolerates this dangling excludesfile path, so OS/editor artifacts may be committed."))
	}

	findings = append(findings, setDiffersFindings(d)...)

	// Check 4: curated gitignore entries.
	// Build a set of existing patterns for O(1) lookup.
	existing := make(map[string]bool, len(state.GitignorePatterns))
	for _, p := range state.GitignorePatterns {
		existing[p] = true
	}
	var missing []string
	for _, p := range gitconfig.DefaultGitignorePatterns() {
		if !existing[p] {
			missing = append(missing, p)
		}
	}
	if len(missing) > 0 {
		// The gitignore restore requires passing the full curated patterns list through
		// the AddWiring dispatcher, which is not supported by the current string-based
		// payload protocol. Fix=nil is correct here (report-only, D-03) — the user
		// must run 'gitid baseline setup' to restore the managed gitignore block.
		// A no-op func() error { return nil } stub is explicitly NOT used (plan advisory).
		findings = append(findings, doctor.Finding{
			Family:       doctor.FamilyBaseline,
			Severity:     doctor.SeverityWarning,
			Title:        "~/.gitignore_global: curated entries missing",
			Explanation:  "One or more gitid-managed gitignore patterns are absent. OS/editor artifacts may be committed.",
			SuggestedFix: "run 'gitid baseline setup' to restore the managed gitignore block",
			Fix:          nil, // report-only: no safe single-call restore via AddWiring (D-03)
		})
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

func matchesDefaultGitignorePatterns(patterns []string) bool {
	defaults := gitconfig.DefaultGitignorePatterns()
	if len(patterns) != len(defaults) {
		return false
	}
	for i, pattern := range defaults {
		if patterns[i] != pattern {
			return false
		}
	}
	return true
}
