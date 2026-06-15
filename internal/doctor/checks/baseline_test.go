package checks

import (
	"strings"
	"testing"

	"github.com/castocolina/gitid/internal/doctor"
	"github.com/castocolina/gitid/internal/gitconfig"
)

// fakeBaselineDeps returns a doctor.Deps with the provided ReadBaselineState
// and path fields for baseline testing. AddWiring is wired with a recording fake
// so that Fix.Fn assertions work correctly in tests that check the [fix] marker.
func fakeBaselineDeps(readFn func(gc, bf, gi string) (gitconfig.BaselineState, error)) doctor.Deps {
	return doctor.Deps{
		GitconfigPath:    "/home/test/.gitconfig",
		BaselineFilePath: "/home/test/.gitconfig.d/00-baseline",
		GitignorePath:    "/home/test/.gitignore_global",
		// AddWiring is wired so that baseline-include Fix.Fn is non-nil.
		// The no-op return is safe here because fakeBaselineDeps is only used
		// to verify the finding shape, not to exercise actual file mutations.
		AddWiring: func(_, _, _ string) error {
			return nil
		},
		ReadBaselineState: readFn,
	}
}

// fullyInstalledState returns a BaselineState representing a fully-configured
// baseline (all four D-16 checks pass).
func fullyInstalledState() gitconfig.BaselineState {
	patterns := gitconfig.DefaultGitignorePatterns()
	return gitconfig.BaselineState{
		Installed: true,
		BaselineKeys: map[string]string{
			"core.excludesfile": "~/.gitignore_global",
			"core.ignorecase":   "false",
		},
		GitignorePatterns: patterns,
	}
}

// TestBaselineAllPass verifies that a fully-configured baseline produces no findings.
func TestBaselineAllPass(t *testing.T) {
	state := fullyInstalledState()
	d := fakeBaselineDeps(func(_, _, _ string) (gitconfig.BaselineState, error) {
		return state, nil
	})
	findings := CheckBaseline(d)
	if len(findings) != 0 {
		t.Errorf("CheckBaseline with fully-configured state: got %d findings, want 0; findings: %v", len(findings), findings)
	}
}

// TestBaselineExcludesfile verifies that an unset or missing excludesfile produces an error finding.
func TestBaselineExcludesfile(t *testing.T) {
	// State where excludesfile is not set.
	state := fullyInstalledState()
	delete(state.BaselineKeys, "core.excludesfile")

	d := fakeBaselineDeps(func(_, _, _ string) (gitconfig.BaselineState, error) {
		return state, nil
	})
	findings := CheckBaseline(d)

	var ef *doctor.Finding
	for i, f := range findings {
		if strings.Contains(f.Title, "excludesfile") {
			ef = &findings[i]
			break
		}
	}
	if ef == nil {
		t.Fatalf("CheckBaseline with excludesfile unset: no finding mentioning 'excludesfile'; got %v", findings)
	}
	if ef.Severity != doctor.SeverityError {
		t.Errorf("excludesfile finding.Severity = %v, want SeverityError", ef.Severity)
	}
	if ef.Family != doctor.FamilyBaseline {
		t.Errorf("excludesfile finding.Family = %q, want %q", ef.Family, doctor.FamilyBaseline)
	}
	if !strings.Contains(ef.SuggestedFix, "gitid baseline setup") {
		t.Errorf("excludesfile finding.SuggestedFix = %q, want it to mention 'gitid baseline setup'", ef.SuggestedFix)
	}
}

// TestBaselineIncludeMissing verifies that a missing baseline include block produces an error finding
// with a Fix descriptor (auto-fixable, re-add class).
func TestBaselineIncludeMissing(t *testing.T) {
	// State where the baseline is not installed (no include block, no baseline block).
	d := fakeBaselineDeps(func(_, _, _ string) (gitconfig.BaselineState, error) {
		return gitconfig.BaselineState{Installed: false}, nil
	})
	findings := CheckBaseline(d)

	var incF *doctor.Finding
	for i, f := range findings {
		if strings.Contains(f.Title, "include") || strings.Contains(f.Title, "baseline") {
			incF = &findings[i]
			break
		}
	}
	if incF == nil {
		t.Fatalf("CheckBaseline with include missing: no finding mentioning 'include'; got %v", findings)
	}
	if incF.Severity != doctor.SeverityError {
		t.Errorf("include missing finding.Severity = %v, want SeverityError", incF.Severity)
	}
	if incF.Family != doctor.FamilyBaseline {
		t.Errorf("include missing finding.Family = %q, want %q", incF.Family, doctor.FamilyBaseline)
	}
	// The include missing finding is auto-fixable (re-add class, D-02) — Fix should be non-nil.
	if incF.Fix == nil {
		t.Error("include missing finding.Fix should be non-nil ([fix] marker)")
	}
	if !strings.Contains(incF.SuggestedFix, "gitid baseline setup") {
		t.Errorf("include missing finding.SuggestedFix = %q, want it to mention 'gitid baseline setup'", incF.SuggestedFix)
	}
}

// TestBaselineIgnoreCaseDrift verifies that core.ignorecase=true produces a warning finding.
func TestBaselineIgnoreCaseDrift(t *testing.T) {
	state := fullyInstalledState()
	state.BaselineKeys["core.ignorecase"] = "true"

	d := fakeBaselineDeps(func(_, _, _ string) (gitconfig.BaselineState, error) {
		return state, nil
	})
	findings := CheckBaseline(d)

	var icF *doctor.Finding
	for i, f := range findings {
		if strings.Contains(f.Title, "ignorecase") {
			icF = &findings[i]
			break
		}
	}
	if icF == nil {
		t.Fatalf("CheckBaseline with ignorecase=true: no finding mentioning 'ignorecase'; got %v", findings)
	}
	if icF.Severity != doctor.SeverityWarning {
		t.Errorf("ignorecase drift finding.Severity = %v, want SeverityWarning", icF.Severity)
	}
	if icF.Family != doctor.FamilyBaseline {
		t.Errorf("ignorecase drift finding.Family = %q, want %q", icF.Family, doctor.FamilyBaseline)
	}
	if !strings.Contains(icF.SuggestedFix, "ignorecase false") {
		t.Errorf("ignorecase drift finding.SuggestedFix = %q, want it to mention setting ignorecase false", icF.SuggestedFix)
	}
}

// TestBaselineCuratedExcludes verifies that a missing curated pattern produces a warning finding.
func TestBaselineCuratedExcludes(t *testing.T) {
	state := fullyInstalledState()
	// Remove all curated patterns from the state — simulates gitignore block missing entries.
	state.GitignorePatterns = nil

	d := fakeBaselineDeps(func(_, _, _ string) (gitconfig.BaselineState, error) {
		return state, nil
	})
	findings := CheckBaseline(d)

	var cF *doctor.Finding
	for i, f := range findings {
		if strings.Contains(f.Title, "curated") || strings.Contains(f.Title, "gitignore") {
			cF = &findings[i]
			break
		}
	}
	if cF == nil {
		t.Fatalf("CheckBaseline with curated excludes missing: no finding mentioning 'curated' or 'gitignore'; got %v", findings)
	}
	if cF.Severity != doctor.SeverityWarning {
		t.Errorf("curated excludes finding.Severity = %v, want SeverityWarning", cF.Severity)
	}
	if cF.Family != doctor.FamilyBaseline {
		t.Errorf("curated excludes finding.Family = %q, want %q", cF.Family, doctor.FamilyBaseline)
	}
	// Curated excludes finding is report-only (Fix=nil) because restoring the gitignore
	// block requires the full curated patterns list, which cannot be safely encoded in
	// the AddWiring string protocol. The user must run 'gitid baseline setup' to restore.
	// A no-op func() error { return nil } stub is explicitly NOT used (plan advisory).
	if cF.Fix != nil {
		t.Error("curated excludes finding.Fix should be nil (report-only — no safe single-call restore)")
	}
	if !strings.Contains(cF.SuggestedFix, "gitid baseline setup") {
		t.Errorf("curated excludes finding.SuggestedFix = %q, want it to mention 'gitid baseline setup'", cF.SuggestedFix)
	}
}

// TestBaselineNilReadFn verifies that nil ReadBaselineState produces no findings.
func TestBaselineNilReadFn(t *testing.T) {
	d := doctor.Deps{
		GitconfigPath:     "/home/test/.gitconfig",
		BaselineFilePath:  "/home/test/.gitconfig.d/00-baseline",
		GitignorePath:     "/home/test/.gitignore_global",
		ReadBaselineState: nil,
	}
	findings := CheckBaseline(d)
	if len(findings) != 0 {
		t.Errorf("CheckBaseline with nil ReadBaselineState: got %d findings, want 0", len(findings))
	}
}
