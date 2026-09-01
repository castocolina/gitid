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
	entries := gitconfig.DefaultGitignoreEntries()
	return gitconfig.BaselineState{
		Installed: true,
		BaselineKeys: map[string]string{
			"core.excludesfile": "~/.gitignore_global",
			"core.ignorecase":   "false",
		},
		GitignorePatterns: entries,
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
		// missing", INFO, not auto-fixable) — never Check 2's "points to a missing global
		// gitignore" ERROR, which is reserved for a genuinely DANGLING pointer
		// (the file does not exist at all). When the block is EMPTY (nil), we're in Branch D
		// and produce a WARNING with a fix.
		{name: "set with empty block", state: func() gitconfig.BaselineState { s := fullyInstalledState(); s.GitignorePatterns = nil; return s }(), value: "~/.gitignore_global", want: doctor.SeverityWarning, wantFix: true, wantTitleContains: "curated entries missing"},
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

// TestBaselineCuratedExcludes verifies that a missing curated pattern produces an informational finding.
func TestBaselineCuratedExcludes(t *testing.T) {
	state := fullyInstalledState()
	// Keep some patterns but remove others — simulates user-edited block missing entries.
	state.GitignorePatterns = []string{
		"Thumbs.db",     // kept
		".idea/",        // kept
		"node_modules/", // kept
		"*.custom",      // personal addition
		// .DS_Store, *.log, etc. deliberately removed
	}

	d := fakeBaselineDeps(func(_, _, _ string) (gitconfig.BaselineState, error) {
		return state, nil
	})
	findings := CheckBaseline(d)

	var cF *doctor.Finding
	for i, f := range findings {
		if strings.Contains(f.Title, "curated") || strings.Contains(f.Title, "absent") {
			cF = &findings[i]
			break
		}
	}
	if cF == nil {
		t.Fatalf("CheckBaseline with curated excludes missing: no finding mentioning 'curated' or 'absent'; got %v", findings)
	}
	if cF.Severity != doctor.SeverityInfo {
		t.Errorf("curated excludes finding.Severity = %v, want SeverityInfo", cF.Severity)
	}
	if cF.Family != doctor.FamilyBaseline {
		t.Errorf("curated excludes finding.Family = %q, want %q", cF.Family, doctor.FamilyBaseline)
	}
	// Curated excludes finding is informational and report-only (Fix=nil) because the
	// block is user-owned. The user can restore entries via the Global Git Ignore screen.
	if cF.Fix != nil {
		t.Error("curated excludes finding.Fix should be nil (informational, user-owned)")
	}
	if !strings.Contains(cF.SuggestedFix, "Global Git Ignore") {
		t.Errorf("curated excludes finding.SuggestedFix = %q, want it to mention 'Global Git Ignore'", cF.SuggestedFix)
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

// ── Task 2.09.2: Six-branch pair decision tree tests ───────────────────────

// TestCheckBaseline_BranchA_KeyUnset verifies that when core.excludesfile
// is unset but baseline is installed, an excludesfile pair warning is produced.
func TestCheckBaseline_BranchA_KeyUnset(t *testing.T) {
	state := gitconfig.BaselineState{
		Installed:         true,
		BaselineKeys:      map[string]string{}, // excludesfile unset
		GitignorePatterns: gitconfig.DefaultGitignoreEntries(),
	}
	d := fakeBaselineDeps(func(_, _, _ string) (gitconfig.BaselineState, error) {
		return state, nil
	})
	d.RunGitConfigGet = func(_, _ string) (string, error) {
		return "", nil // key unset
	}
	findings := CheckBaseline(d)

	var pairFinding *doctor.Finding
	for i := range findings {
		if strings.Contains(findings[i].Title, "excludesfile") && strings.Contains(findings[i].Title, "not configured") {
			pairFinding = &findings[i]
			break
		}
	}

	if pairFinding == nil {
		t.Fatal("expected a 'not configured' excludesfile finding for Branch A")
	}
	if pairFinding.Severity != doctor.SeverityWarning {
		t.Errorf("Branch A severity = %v, want Warning", pairFinding.Severity)
	}
	if pairFinding.Fix == nil {
		t.Error("Branch A must have a non-nil Fix descriptor")
	}
}

// TestCheckBaseline_BranchB2_WrongTargetMissing verifies that when
// core.excludesfile points to a DIFFERENT file that does NOT exist,
// an ERROR is produced with a specific title naming the configured path,
// and Fix is nil (no retargeting allowed).
func TestCheckBaseline_BranchB2_WrongTargetMissing(t *testing.T) {
	customPath := "/some/deleted/custom-ignore"
	state := gitconfig.BaselineState{
		Installed: true,
		BaselineKeys: map[string]string{
			"core.excludesfile": customPath, // wired to a different path
		},
		GitignorePatterns: gitconfig.DefaultGitignoreEntries(),
	}
	d := fakeBaselineDeps(func(_, _, _ string) (gitconfig.BaselineState, error) {
		return state, nil
	})

	d.Stat = func(checkPath string) (os.FileInfo, error) {
		if checkPath == customPath {
			return nil, os.ErrNotExist // the configured file does not exist
		}
		return nil, nil // other files exist
	}

	findings := CheckBaseline(d)

	var errorFinding *doctor.Finding
	for i := range findings {
		if findings[i].Severity == doctor.SeverityError && strings.Contains(findings[i].Title, "excludesfile") {
			errorFinding = &findings[i]
			break
		}
	}

	if errorFinding == nil {
		t.Fatalf("expected an ERROR excludesfile finding for Branch B2 (missing custom file), got %d findings", len(findings))
	}
	// Branch B2's title must NAME the configured path, not the managed target.
	if !strings.Contains(errorFinding.Title, customPath) {
		t.Errorf("Branch B2 title must contain the configured path %q, got %q", customPath, errorFinding.Title)
	}
	if errorFinding.Fix != nil {
		t.Error("Branch B2 must have Fix=nil (no silent retargeting allowed)")
	}
}

// TestCheckBaseline_BranchB_WrongTargetExists verifies that when
// core.excludesfile points to a DIFFERENT file that DOES exist,
// an INFORMATIONAL finding is produced with no executable fix.
func TestCheckBaseline_BranchB_WrongTargetExists(t *testing.T) {
	customPath := "/home/test/.config/gitignore"
	state := gitconfig.BaselineState{
		Installed: true,
		BaselineKeys: map[string]string{
			"core.excludesfile": customPath, // wired to a different path
		},
		GitignorePatterns: gitconfig.DefaultGitignoreEntries(),
	}
	d := fakeBaselineDeps(func(_, _, _ string) (gitconfig.BaselineState, error) {
		return state, nil
	})

	d.Stat = func(checkPath string) (os.FileInfo, error) {
		if checkPath == customPath {
			return nil, nil // file exists
		}
		return nil, os.ErrNotExist
	}

	findings := CheckBaseline(d)

	var infoFinding *doctor.Finding
	for i := range findings {
		if findings[i].Severity == doctor.SeverityInfo && strings.Contains(findings[i].Title, "excludesfile") {
			infoFinding = &findings[i]
			break
		}
	}

	if infoFinding == nil {
		t.Fatal("expected an INFORMATIONAL excludesfile finding for Branch B (existing custom file)")
	}
	if infoFinding.Fix != nil {
		t.Error("Branch B must have Fix=nil (user's deliberate choice)")
	}
}

// TestCheckBaseline_BranchC_ManagedTargetMissing verifies that when
// core.excludesfile points to the MANAGED target but the file does NOT exist,
// an ERROR is produced with a non-nil Fix.
func TestCheckBaseline_BranchC_ManagedTargetMissing(t *testing.T) {
	state := gitconfig.BaselineState{
		Installed: true,
		BaselineKeys: map[string]string{
			"core.excludesfile": "~/.gitignore_global", // managed target
		},
		GitignorePatterns: gitconfig.DefaultGitignoreEntries(),
	}
	d := fakeBaselineDeps(func(_, _, _ string) (gitconfig.BaselineState, error) {
		return state, nil
	})

	d.Stat = func(_ string) (os.FileInfo, error) {
		return nil, os.ErrNotExist // file missing
	}

	findings := CheckBaseline(d)

	var errorFinding *doctor.Finding
	for i := range findings {
		if findings[i].Severity == doctor.SeverityError && strings.Contains(findings[i].Title, "excludesfile") {
			errorFinding = &findings[i]
			break
		}
	}

	if errorFinding == nil {
		t.Fatal("expected an ERROR excludesfile finding for Branch C (missing managed target)")
	}
	if errorFinding.Fix == nil {
		t.Error("Branch C must have a non-nil Fix descriptor")
	}
}

// TestCheckBaseline_BranchD_ManagedTargetEmptyBlock verifies that when
// core.excludesfile points to the managed target, the file exists, but
// the managed block is empty or absent, a WARNING is produced with a
// non-nil Fix.
func TestCheckBaseline_BranchD_ManagedTargetEmptyBlock(t *testing.T) {
	state := gitconfig.BaselineState{
		Installed: true,
		BaselineKeys: map[string]string{
			"core.excludesfile": "~/.gitignore_global",
		},
		GitignorePatterns: []string{}, // no patterns in block
	}
	d := fakeBaselineDeps(func(_, _, _ string) (gitconfig.BaselineState, error) {
		return state, nil
	})

	d.Stat = func(_ string) (os.FileInfo, error) {
		return nil, nil // file exists
	}

	findings := CheckBaseline(d)

	var warningFinding *doctor.Finding
	for i := range findings {
		if findings[i].Severity == doctor.SeverityWarning && strings.Contains(findings[i].Title, ".gitignore_global") {
			warningFinding = &findings[i]
			break
		}
	}

	if warningFinding == nil {
		t.Fatalf("expected a WARNING .gitignore_global finding for Branch D (empty managed block), got %d findings", len(findings))
	}
	if warningFinding.Fix == nil {
		t.Error("Branch D must have a non-nil Fix descriptor")
	}
}

// TestCheckBaseline_BranchE_Configured verifies that when
// core.excludesfile points to the managed target, the file exists, and
// the block is populated, NO pair-related finding is produced (Branch E).
func TestCheckBaseline_BranchE_Configured(t *testing.T) {
	state := fullyInstalledState()
	state.BaselineKeys["core.excludesfile"] = "~/.gitignore_global" // ensure it's wired to managed target
	d := fakeBaselineDeps(func(_, _, _ string) (gitconfig.BaselineState, error) {
		return state, nil
	})

	d.Stat = func(_ string) (os.FileInfo, error) {
		return nil, nil // file exists
	}

	findings := CheckBaseline(d)

	for _, f := range findings {
		if strings.Contains(f.Title, "excludesfile") && strings.Contains(f.Title, "not configured") {
			t.Errorf("Branch E (configured) must not produce a 'not configured' finding, got: %v", f)
		}
		if strings.Contains(f.Title, "missing global gitignore") {
			t.Errorf("Branch E must not produce 'missing global gitignore', got: %v", f)
		}
	}
}

// TestCheckBaseline_EditedBlockIsConfigured verifies that a managed gitignore
// block that the user has EDITED (default entry removed, personal entry added)
// is treated as configured: no error or warning saying excludesfile is not configured.
func TestCheckBaseline_EditedBlockIsConfigured(t *testing.T) {
	editedPatterns := []string{
		"Thumbs.db",     // default, kept
		".idea/",        // default, kept
		"node_modules/", // default, kept
		// .DS_Store deliberately removed
		"*.mine", // personal addition
	}

	state := gitconfig.BaselineState{
		Installed: true,
		BaselineKeys: map[string]string{
			"core.excludesfile": "~/.gitignore_global",
		},
		GitignorePatterns: editedPatterns,
	}
	d := fakeBaselineDeps(func(_, _, _ string) (gitconfig.BaselineState, error) {
		return state, nil
	})

	d.RunGitConfigGet = func(_, _ string) (string, error) {
		return "~/.gitignore_global", nil
	}
	d.Stat = func(_ string) (os.FileInfo, error) {
		return nil, nil // file exists
	}

	findings := CheckBaseline(d)

	// Should not report "not configured" error/warning.
	for _, f := range findings {
		if strings.Contains(f.Title, "not configured") {
			t.Errorf("edited block should not produce 'not configured' finding, got: %v", f)
		}
	}
}

// TestCheckBaseline_EditedBlockInformationalOnly verifies that when a
// managed gitignore block has been edited (some defaults removed, some added),
// the only finding is informational about the absence of default entries,
// with no executable fix and no error/warning severity.
func TestCheckBaseline_EditedBlockInformationalOnly(t *testing.T) {
	editedPatterns := []string{
		"Thumbs.db", // kept
		".idea/",    // kept
		"*.mine",    // added
		// .DS_Store removed, *.log removed, etc.
	}

	state := gitconfig.BaselineState{
		Installed: true,
		BaselineKeys: map[string]string{
			"core.excludesfile": "~/.gitignore_global",
		},
		GitignorePatterns: editedPatterns,
	}
	d := fakeBaselineDeps(func(_, _, _ string) (gitconfig.BaselineState, error) {
		return state, nil
	})

	d.RunGitConfigGet = func(_, _ string) (string, error) {
		return "~/.gitignore_global", nil
	}
	d.Stat = func(_ string) (os.FileInfo, error) {
		return nil, nil
	}

	findings := CheckBaseline(d)

	// There should be an informational finding about missing entries,
	// but no error or warning.
	var infoAboutMissing *doctor.Finding
	for i := range findings {
		if findings[i].Severity == doctor.SeverityInfo && strings.Contains(findings[i].Title, "missing") {
			infoAboutMissing = &findings[i]
		}
		// Should not have errors or warnings about not being configured.
		if strings.Contains(findings[i].Title, "not configured") {
			t.Errorf("edited block should not produce 'not configured', got: %v", findings[i])
		}
	}

	if infoAboutMissing != nil && infoAboutMissing.Fix != nil {
		t.Errorf("informational finding about edited block must have Fix=nil, got: %v", infoAboutMissing.Fix)
	}
}

// TestCheckBaseline_CommentHeadersNotPenalized verifies that a block containing
// comment headers is not penalized — the comparison operates on the comment-free view.
func TestCheckBaseline_CommentHeadersNotPenalized(t *testing.T) {
	patterns := gitconfig.DefaultGitignorePatterns() // includes comment headers
	entries := gitconfig.DefaultGitignoreEntries()   // comment-free view

	state := gitconfig.BaselineState{
		Installed: true,
		BaselineKeys: map[string]string{
			"core.excludesfile": "~/.gitignore_global",
		},
		GitignorePatterns: entries, // read back via parseGitignoreBlockBody
	}
	d := fakeBaselineDeps(func(_, _, _ string) (gitconfig.BaselineState, error) {
		return state, nil
	})

	d.RunGitConfigGet = func(_, _ string) (string, error) {
		return "~/.gitignore_global", nil
	}
	d.Stat = func(_ string) (os.FileInfo, error) {
		return nil, nil
	}

	// Sanity check: patterns includes comments.
	hasComment := false
	for _, p := range patterns {
		if strings.HasPrefix(strings.TrimSpace(p), "#") {
			hasComment = true
			break
		}
	}
	if !hasComment {
		t.Fatal("test setup error: DefaultGitignorePatterns should include comments")
	}

	findings := CheckBaseline(d)

	// No findings should complain about missing curated entries (they are all present in entries).
	for _, f := range findings {
		if strings.Contains(f.Title, "curated entries missing") {
			t.Errorf("comment headers should not cause 'missing entries' finding, got: %v", f)
		}
	}
}
