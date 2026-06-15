// Package checks implements the per-family health check functions for
// gitid doctor. Each family lives in its own file and is overwritten in
// place by Wave 2 plans without redeclaration.
package checks

import (
	"strings"

	"github.com/castocolina/gitid/internal/doctor"
	"github.com/castocolina/gitid/internal/gitconfig"
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
		findings = append(findings, doctor.Finding{
			Family:       doctor.FamilyBaseline,
			Severity:     doctor.SeverityError,
			Title:        "baseline [include] block missing from ~/.gitconfig",
			Explanation:  "The managed baseline include block is gone. Baseline settings have no effect.",
			SuggestedFix: "run 'gitid baseline setup'",
			Fix: &doctor.FixDescriptor{
				Summary: "run 'gitid baseline setup'",
				// The actual fixer is wired by Plan 05; here we mark it fixable.
				Fn: func() error { return nil },
			},
		})
		// When the baseline is not installed, the other checks cannot be meaningful
		// (there are no keys to read). Return early with just this finding.
		return findings
	}

	// Check 2: core.excludesfile wiring.
	// The key must be present in BaselineKeys. An empty value or absent key is reported.
	excludesFile, excludesSet := state.BaselineKeys["core.excludesfile"]
	if !excludesSet || strings.TrimSpace(excludesFile) == "" {
		findings = append(findings, doctor.Finding{
			Family:       doctor.FamilyBaseline,
			Severity:     doctor.SeverityError,
			Title:        "core.excludesfile: not set or file missing",
			Explanation:  "The global gitignore is not configured. OS/editor artifacts will not be excluded.",
			SuggestedFix: "run 'gitid baseline setup'",
			Fix:          nil, // excludesfile wiring is re-run not auto-patchable individually
		})
	}

	// Check 3: core.ignorecase locked-value carve-out (D-17).
	// The baseline sets ignorecase=false; if something flipped it to true, warn.
	if icVal, ok := state.BaselineKeys["core.ignorecase"]; ok && icVal != "false" {
		findings = append(findings, doctor.Finding{
			Family:       doctor.FamilyBaseline,
			Severity:     doctor.SeverityWarning,
			Title:        "core.ignorecase: " + icVal + " (expected false)",
			Explanation:  "An override has enabled case-insensitive matching. This can hide filename case conflicts on macOS.",
			SuggestedFix: "git config --global core.ignorecase false  or re-run 'gitid baseline setup'",
			Fix:          nil, // locked-value override is report-only; user must decide
		})
	}

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
		findings = append(findings, doctor.Finding{
			Family:       doctor.FamilyBaseline,
			Severity:     doctor.SeverityWarning,
			Title:        "~/.gitignore_global: curated entries missing",
			Explanation:  "One or more gitid-managed gitignore patterns are absent. OS/editor artifacts may be committed.",
			SuggestedFix: "run 'gitid baseline setup' to restore the managed gitignore block",
			Fix: &doctor.FixDescriptor{
				Summary: "run 'gitid baseline setup' to restore the managed gitignore block",
				// The actual fixer is wired by Plan 05; here we mark it fixable.
				Fn: func() error { return nil },
			},
		})
	}

	return findings
}
