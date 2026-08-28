package checks

import (
	"os"
	"strings"
	"testing"

	"github.com/castocolina/gitid/internal/doctor"
	"github.com/castocolina/gitid/internal/gitconfig"
	"github.com/castocolina/gitid/internal/globalgit"
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
		RunGitConfigGet: func(_, key string) (string, error) {
			if key == "core.excludesfile" {
				return "~/.gitignore_global", nil
			}
			return "", nil
		},
		FixExcludesfile: func(string) error {
			return nil
		},
		Stat: func(string) (os.FileInfo, error) {
			return nil, nil
		},
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

func TestCheckBaselineGitignorePair(t *testing.T) {
	tests := []struct {
		name              string
		state             gitconfig.BaselineState
		value             string
		want              doctor.Severity
		wantFix           bool
		wantTitleContains string
	}{
		{name: "unset with no managed patterns", state: gitconfig.BaselineState{Installed: true, BaselineKeys: map[string]string{}}, want: doctor.SeverityWarning, wantFix: true},
		// A wired excludesfile whose pattern FILE EXISTS but whose content differs
		// from the curated defaults is Check 4's territory ("curated entries
		// missing", WARNING) — never Check 2's "points to a missing global
		// gitignore" ERROR, which is reserved for a genuinely DANGLING pointer
		// (the file does not exist at all). Conflating the two would misreport
		// an existing-but-incomplete file as absent (found empirically: this
		// test originally asserted the WRONG, over-broad ERROR behavior).
		{name: "set with missing patterns", state: func() gitconfig.BaselineState { s := fullyInstalledState(); s.GitignorePatterns = nil; return s }(), value: "~/.gitignore_global", want: doctor.SeverityWarning, wantFix: false, wantTitleContains: "curated entries missing"},
		{name: "correct pair", state: fullyInstalledState(), value: "~/.gitignore_global"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := fakeBaselineDeps(func(_, _, _ string) (gitconfig.BaselineState, error) { return tt.state, nil })
			d.RunGitConfigGet = func(_, _ string) (string, error) { return tt.value, nil }
			findings := CheckBaseline(d)
			titleSubstr := "excludesfile"
			if tt.wantTitleContains != "" {
				titleSubstr = tt.wantTitleContains
			}
			var got *doctor.Finding
			for i := range findings {
				if strings.Contains(findings[i].Title, titleSubstr) {
					got = &findings[i]
					break
				}
			}
			if tt.want == 0 {
				if got != nil {
					t.Fatalf("unexpected gitignore finding: %+v", *got)
				}
				return
			}
			if got == nil {
				t.Fatal("missing gitignore finding")
			}
			if got.Severity != tt.want {
				t.Errorf("Severity = %v, want %v", got.Severity, tt.want)
			}
			// Check 4's inline Finding literal (the "curated entries missing"
			// fallback path, matched via wantTitleContains) omits Target,
			// relying on doctor.Run()'s defaultTargetForFamily(FamilyBaseline)
			// resolution — CheckBaseline is called directly here, bypassing
			// Run(), so Target is legitimately empty only in that one case.
			if tt.wantTitleContains == "" && got.Target != "Git" {
				t.Errorf("Target = %q, want Git", got.Target)
			}
			if (got.Fix != nil) != tt.wantFix {
				t.Errorf("Fix present = %v, want %v", got.Fix != nil, tt.wantFix)
			}
		})
	}
}

// TestFixExcludesfileCallsInjectedEffect uses the "excludesfile entirely
// unset, no managed pattern file" scenario — Check 2's WARNING branch,
// which carries a Fix (unlike the "file exists but content differs"
// scenario, which is Check 4's report-only WARNING, Fix: nil — see
// TestCheckBaselineGitignorePair's "set with missing patterns" case).
func TestFixExcludesfileCallsInjectedEffect(t *testing.T) {
	state := gitconfig.BaselineState{Installed: true, BaselineKeys: map[string]string{}}
	d := fakeBaselineDeps(func(_, _, _ string) (gitconfig.BaselineState, error) { return state, nil })
	calledPath := ""
	d.FixExcludesfile = func(path string) error { calledPath = path; return nil }
	finding := CheckBaseline(d)[0]
	if finding.Fix == nil {
		t.Fatal("Fix is nil")
	}
	if err := finding.Fix.Fn(); err != nil {
		t.Fatalf("Fix.Fn: %v", err)
	}
	if calledPath != d.GitignorePath {
		t.Errorf("FixExcludesfile path = %q, want %q", calledPath, d.GitignorePath)
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

func TestCheckBaselineSetDiffers(t *testing.T) {
	state := fullyInstalledState()
	d := fakeBaselineDeps(func(_, _, _ string) (gitconfig.BaselineState, error) { return state, nil })
	d.RunGitConfigGet = func(_, key string) (string, error) {
		if key == "init.defaultBranch" {
			return "trunk", nil
		}
		return "", nil
	}
	findings := CheckBaseline(d)
	var got *doctor.Finding
	for i := range findings {
		if strings.Contains(findings[i].Title, "init.defaultBranch") {
			got = &findings[i]
			break
		}
	}
	if got == nil {
		t.Fatal("missing set-differs finding")
	}
	if got.Severity != doctor.SeverityInfo {
		t.Errorf("Severity = %v, want SeverityInfo", got.Severity)
	}
	if got.Fix != nil {
		t.Errorf("Fix = %+v, want nil", got.Fix)
	}
}

func TestSeverityNeverEscalates(t *testing.T) {
	for _, policy := range globalgit.Policy {
		for _, member := range policy.Members {
			t.Run(member.Key, func(t *testing.T) {
				value := member.Recommended + "-user-choice"
				d := fakeBaselineDeps(func(_, _, _ string) (gitconfig.BaselineState, error) { return fullyInstalledState(), nil })
				d.RunGitConfigGet = func(_, key string) (string, error) {
					if key == member.Key {
						return value, nil
					}
					return "", nil
				}
				var found bool
				for _, finding := range setDiffersFindings(d) {
					if finding.Title == member.Key+": "+value+" (differs from recommendation)" {
						found = true
						if finding.Severity != doctor.SeverityInfo {
							t.Fatalf("Severity = %v, want SeverityInfo", finding.Severity)
						}
					}
				}
				if !found {
					t.Fatalf("missing set-differs finding for %q", member.Key)
				}
			})
		}
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
		if strings.Contains(f.Title, "curated") {
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
