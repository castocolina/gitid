package doctor_test

import (
	"testing"

	"github.com/castocolina/gitid/internal/doctor"
	"github.com/castocolina/gitid/internal/doctor/checks"
)

// TestSeverityString verifies that each Severity level produces its canonical
// lowercase label string.
func TestSeverityString(t *testing.T) {
	cases := []struct {
		sev  doctor.Severity
		want string
	}{
		{doctor.SeverityInfo, "info"},
		{doctor.SeverityWarning, "warning"},
		{doctor.SeverityError, "error"},
		{doctor.SeverityCritical, "critical"},
	}
	for _, c := range cases {
		if got := c.sev.String(); got != c.want {
			t.Errorf("Severity(%d).String() = %q, want %q", int(c.sev), got, c.want)
		}
	}
}

// TestExitCodeClean verifies that a nil findings slice returns exit code 0.
func TestExitCodeClean(t *testing.T) {
	if got := doctor.ExitCode(nil); got != 0 {
		t.Errorf("ExitCode(nil) = %d, want 0", got)
	}
}

// TestExitCodeInfo verifies that info-only findings return exit code 1.
func TestExitCodeInfo(t *testing.T) {
	findings := []doctor.Finding{
		{Severity: doctor.SeverityInfo},
	}
	if got := doctor.ExitCode(findings); got != 1 {
		t.Errorf("ExitCode([info]) = %d, want 1", got)
	}
}

// TestExitCodeWarning verifies that warning findings return exit code 1.
func TestExitCodeWarning(t *testing.T) {
	findings := []doctor.Finding{
		{Severity: doctor.SeverityWarning},
	}
	if got := doctor.ExitCode(findings); got != 1 {
		t.Errorf("ExitCode([warning]) = %d, want 1", got)
	}
}

// TestExitCodeError verifies that error-level findings return exit code 2.
func TestExitCodeError(t *testing.T) {
	findings := []doctor.Finding{
		{Severity: doctor.SeverityError},
	}
	if got := doctor.ExitCode(findings); got != 2 {
		t.Errorf("ExitCode([error]) = %d, want 2", got)
	}
}

// TestExitCodeCritical verifies that critical findings return exit code 3.
func TestExitCodeCritical(t *testing.T) {
	findings := []doctor.Finding{
		{Severity: doctor.SeverityCritical},
	}
	if got := doctor.ExitCode(findings); got != 3 {
		t.Errorf("ExitCode([critical]) = %d, want 3", got)
	}
}

// TestExitCodeHighestWins verifies that the highest severity in a mixed slice
// determines the exit code (info + critical → 3).
func TestExitCodeHighestWins(t *testing.T) {
	findings := []doctor.Finding{
		{Severity: doctor.SeverityInfo},
		{Severity: doctor.SeverityCritical},
		{Severity: doctor.SeverityWarning},
	}
	if got := doctor.ExitCode(findings); got != 3 {
		t.Errorf("ExitCode([info,critical,warning]) = %d, want 3 (highest-wins)", got)
	}
}

// TestFindingFields verifies that a Finding struct carries its expected fields
// (Family, Severity, Title, Explanation, SuggestedFix) and that Fix can be
// nil (report-only) or non-nil (auto-fixable).
func TestFindingFields(t *testing.T) {
	f := doctor.Finding{
		Family:       doctor.FamilyPerms,
		Severity:     doctor.SeverityCritical,
		Title:        "~/.ssh/key: 644 (expected 600)",
		Explanation:  "Private key has group/world read permission.",
		SuggestedFix: "chmod 0600 ~/.ssh/key",
		Fix:          nil,
	}
	if f.Family == "" {
		t.Error("Family must be non-empty")
	}
	if f.Title == "" {
		t.Error("Title must be non-empty")
	}
	if f.Explanation == "" {
		t.Error("Explanation must be non-empty")
	}
	if f.SuggestedFix == "" {
		t.Error("SuggestedFix must be non-empty")
	}
	// Fix nil means report-only — that is valid.
	if f.Fix != nil {
		t.Error("Fix expected nil for report-only finding")
	}

	// Non-nil Fix with a callable Fn.
	var called bool
	fd := &doctor.FixDescriptor{
		Summary: "chmod 0600 ~/.ssh/key",
		Fn:      func() error { called = true; return nil },
	}
	f.Fix = fd
	if err := f.Fix.Fn(); err != nil {
		t.Fatalf("FixDescriptor.Fn() returned error: %v", err)
	}
	if !called {
		t.Error("FixDescriptor.Fn was not invoked")
	}
}

// TestRunCallsAllFamilies verifies that Run dispatches to the injected check
// function and returns its findings. The six stub families contribute nothing.
func TestRunCallsAllFamilies(t *testing.T) {
	wantFinding := doctor.Finding{
		Family:   doctor.FamilyPerms,
		Severity: doctor.SeverityCritical,
		Title:    "fake critical finding",
	}
	fakeDeps := doctor.Deps{
		CheckPerms: func(_ doctor.Deps) []doctor.Finding {
			return []doctor.Finding{wantFinding}
		},
	}
	findings := doctor.Run(fakeDeps)
	if len(findings) != 1 {
		t.Fatalf("Run() returned %d findings, want 1", len(findings))
	}
	if findings[0].Title != wantFinding.Title {
		t.Errorf("Run() finding title = %q, want %q", findings[0].Title, wantFinding.Title)
	}
}

// TestFamiliesFixedOrder verifies that Families() returns the 8 family
// constants in the UI-SPEC fixed order: Dependencies, Permissions, Coherence,
// Orphans, Signing, Agent, Baseline, Overlap.
func TestFamiliesFixedOrder(t *testing.T) {
	want := []doctor.Family{
		doctor.FamilyDeps,
		doctor.FamilyPerms,
		doctor.FamilyCoherence,
		doctor.FamilyOrphans,
		doctor.FamilySigning,
		doctor.FamilyAgent,
		doctor.FamilyBaseline,
		doctor.FamilyOverlap,
	}
	got := doctor.Families()
	if len(got) != len(want) {
		t.Fatalf("Families() len = %d, want %d; got %v", len(got), len(want), got)
	}
	for i, w := range want {
		if got[i] != w {
			t.Errorf("Families()[%d] = %q, want %q", i, got[i], w)
		}
	}
}

// TestCheckFunctionSignatures confirms the checks package exports the required
// function signatures (compile-time check; stub bodies returning nil).
func TestCheckFunctionSignatures(t *testing.T) {
	var deps doctor.Deps
	// All six stub families must compile and return nil.
	if got := checks.CheckDeps(deps); got != nil {
		t.Errorf("CheckDeps stub must return nil, got %v", got)
	}
	if got := checks.CheckBaseline(deps); got != nil {
		t.Errorf("CheckBaseline stub must return nil, got %v", got)
	}
	if got := checks.CheckCoherence(deps); got != nil {
		t.Errorf("CheckCoherence stub must return nil, got %v", got)
	}
	if got := checks.CheckOrphans(deps); got != nil {
		t.Errorf("CheckOrphans stub must return nil, got %v", got)
	}
	if got := checks.CheckSigning(deps); got != nil {
		t.Errorf("CheckSigning stub must return nil, got %v", got)
	}
	if got := checks.CheckAgent(deps); got != nil {
		t.Errorf("CheckAgent stub must return nil, got %v", got)
	}
}
