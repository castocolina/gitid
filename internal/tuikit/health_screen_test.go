package tuikit

import (
	"strings"
	"testing"
)

func TestParseErrorScreenRequiresFilesFamily(t *testing.T) {
	files := DemoFinding{HealthFinding: HealthFinding{Family: "Files", Severity: SeverityCritical, Section: "Git", Title: "Git configuration cannot be parsed", Explanation: "bad config"}}
	if _, ok := parseErrorFinding([]DemoFinding{files}); !ok {
		t.Fatal("Files-critical finding must select the parse-error frame")
	}
	perms := DemoFinding{HealthFinding: HealthFinding{Family: "Permissions", Severity: SeverityCritical, Section: "SSH", Title: "private key exposed"}}
	if _, ok := parseErrorFinding([]DemoFinding{perms}); ok {
		t.Fatal("Permissions-critical finding must not select the parse-error frame")
	}
}

func TestHealthParseErrorFrameSuppressesOrdinaryFindings(t *testing.T) {
	m := newHealthModel()
	state := DemoState{Scanned: true, Findings: []DemoFinding{
		{HealthFinding: HealthFinding{Family: "Files", Severity: SeverityCritical, Section: "Git", Title: "Git configuration cannot be parsed", Explanation: "bad config"}},
		{HealthFinding: HealthFinding{Family: "Coherence", Severity: SeverityError, Section: "Git", Title: "misleading derived finding"}},
	}}
	view := m.view(state, 100, 30)
	if !strings.Contains(view.body, "Checks paused") || strings.Contains(view.body, "misleading derived finding") {
		t.Fatalf("parse-error frame = %q", view.body)
	}
}

func TestHealthNegativeAssertion(t *testing.T) {
	withFindings := Seed()
	withFindings.Scanned = true
	perIdentity := withFindings
	parseError := DemoState{Scanned: true, Findings: []DemoFinding{{
		HealthFinding: HealthFinding{Family: "Files", Severity: SeverityCritical, Section: "Git", Title: "Git configuration cannot be parsed", Explanation: "bad config"},
		Identity:      "legacy",
	}}}
	for name, tc := range map[string]struct {
		model healthModel
		state DemoState
	}{
		"with-findings": {model: newHealthModel(), state: withFindings},
		"all-green":     {model: newHealthModel(), state: DemoState{Scanned: true}},
		"per-identity":  {model: healthModel{identityName: "legacy"}, state: perIdentity},
		"parse-error":   {model: newHealthModel(), state: parseError},
	} {
		t.Run(name, func(t *testing.T) {
			view := stripANSI(tc.model.view(tc.state, 100, 30).body)
			for _, forbidden := range []string{"Apply fix", "Confirm write", "Backed up ->", "Wrote ->", "f ·"} {
				if strings.Contains(view, forbidden) {
					t.Errorf("Health rendered forbidden write marker %q:\n%s", forbidden, view)
				}
			}
		})
	}
}
