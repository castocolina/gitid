package checks

import (
	"strings"
	"testing"

	"github.com/castocolina/gitid/internal/deps"
	"github.com/castocolina/gitid/internal/doctor"
)

// fakeDetectTools builds a fake DetectTools closure returning the provided Report.
func fakeDetectTools(r deps.Report) func() deps.Report {
	return func() deps.Report { return r }
}

// fakeInstallHint returns a predictable hint for testing.
func fakeInstallHint(tool, _ string) string {
	return "brew install " + tool
}

// fakeCurrentOS returns "darwin" for tests.
func fakeCurrentOS() string { return "darwin" }

// fakeDepsDeps builds a doctor.Deps with all required fields for CheckDeps tests.
func fakeDepsDeps(report deps.Report) doctor.Deps {
	return doctor.Deps{
		DetectTools: fakeDetectTools(report),
		InstallHint: fakeInstallHint,
		CurrentOS:   fakeCurrentOS,
	}
}

// TestCheckDeps_AllPresent verifies that no findings are produced when all tools are present.
func TestCheckDeps_AllPresent(t *testing.T) {
	d := fakeDepsDeps(deps.Report{
		SSH:       true,
		SSHKeygen: true,
		SSHAdd:    true,
		Git:       true,
		Clipboard: true,
	})
	findings := CheckDeps(d)
	if len(findings) != 0 {
		t.Errorf("CheckDeps with all tools present: got %d findings, want 0; findings: %v", len(findings), findings)
	}
}

// TestCheckDeps_RequiredMissing verifies that a missing required tool produces an error finding.
func TestCheckDeps_RequiredMissing(t *testing.T) {
	// Only SSH is missing; others present.
	d := fakeDepsDeps(deps.Report{
		SSH:       false,
		SSHKeygen: true,
		SSHAdd:    true,
		Git:       true,
		Clipboard: true,
	})
	findings := CheckDeps(d)

	if len(findings) != 1 {
		t.Fatalf("CheckDeps with ssh missing: got %d findings, want 1; findings: %v", len(findings), findings)
	}

	f := findings[0]
	if f.Family != doctor.FamilyDeps {
		t.Errorf("finding.Family = %q, want %q", f.Family, doctor.FamilyDeps)
	}
	if f.Severity != doctor.SeverityError {
		t.Errorf("finding.Severity = %v, want SeverityError", f.Severity)
	}
	if !strings.Contains(f.Title, "ssh") {
		t.Errorf("finding.Title = %q, want it to mention 'ssh'", f.Title)
	}
	if !strings.Contains(f.SuggestedFix, "brew install ssh") {
		t.Errorf("finding.SuggestedFix = %q, want it to contain install hint for ssh", f.SuggestedFix)
	}
	// Dep installs are report-only — no Fix descriptor (D-03).
	if f.Fix != nil {
		t.Error("finding.Fix should be nil for report-only dep findings (D-03)")
	}
}

// TestCheckDeps_MultipleRequiredMissing verifies that each missing required tool
// produces its own error finding.
func TestCheckDeps_MultipleRequiredMissing(t *testing.T) {
	d := fakeDepsDeps(deps.Report{
		SSH:       false,
		SSHKeygen: false,
		SSHAdd:    true,
		Git:       false,
		Clipboard: true,
	})
	findings := CheckDeps(d)

	// ssh, ssh-keygen, and git are required; each should produce one error finding.
	if len(findings) != 3 {
		t.Fatalf("CheckDeps with ssh+ssh-keygen+git missing: got %d findings, want 3", len(findings))
	}
	for _, f := range findings {
		if f.Severity != doctor.SeverityError {
			t.Errorf("required missing finding.Severity = %v, want SeverityError", f.Severity)
		}
		if f.Family != doctor.FamilyDeps {
			t.Errorf("required missing finding.Family = %q, want %q", f.Family, doctor.FamilyDeps)
		}
	}
}

// TestCheckDeps_OptionalClipboardMissing verifies that a missing clipboard tool
// produces an info finding (not an error), with the correct glyph convention.
func TestCheckDeps_OptionalClipboardMissing(t *testing.T) {
	d := fakeDepsDeps(deps.Report{
		SSH:       true,
		SSHKeygen: true,
		SSHAdd:    true,
		Git:       true,
		Clipboard: false,
	})
	findings := CheckDeps(d)

	if len(findings) != 1 {
		t.Fatalf("CheckDeps with clipboard missing: got %d findings, want 1", len(findings))
	}
	f := findings[0]
	if f.Severity != doctor.SeverityInfo {
		t.Errorf("optional clipboard finding.Severity = %v, want SeverityInfo", f.Severity)
	}
	if f.Family != doctor.FamilyDeps {
		t.Errorf("optional clipboard finding.Family = %q, want %q", f.Family, doctor.FamilyDeps)
	}
	// The explanation must mention "Public-key copy to clipboard" per UI-SPEC.
	if !strings.Contains(f.Explanation, "Public-key copy to clipboard") {
		t.Errorf("optional clipboard finding.Explanation = %q, want it to contain 'Public-key copy to clipboard'", f.Explanation)
	}
	// No Fix descriptor for dep installs (D-03).
	if f.Fix != nil {
		t.Error("clipboard finding.Fix should be nil (report-only, D-03)")
	}
}

// TestCheckDeps_SSHAddMissingIsRequired verifies that missing ssh-add is treated as required (error).
func TestCheckDeps_SSHAddMissingIsRequired(t *testing.T) {
	d := fakeDepsDeps(deps.Report{
		SSH:       true,
		SSHKeygen: true,
		SSHAdd:    false,
		Git:       true,
		Clipboard: true,
	})
	findings := CheckDeps(d)

	if len(findings) != 1 {
		t.Fatalf("CheckDeps with ssh-add missing: got %d findings, want 1", len(findings))
	}
	f := findings[0]
	if f.Severity != doctor.SeverityError {
		t.Errorf("ssh-add missing finding.Severity = %v, want SeverityError", f.Severity)
	}
}
