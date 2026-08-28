package main

import (
	"testing"

	"github.com/castocolina/gitid/internal/tuikit"
)

func TestHealthJSONParseErrorSuppression(t *testing.T) {
	findings := []tuikit.DemoFinding{
		{HealthFinding: tuikit.HealthFinding{Family: "Files", Severity: tuikit.SeverityCritical, Section: "Git", Title: "Git configuration cannot be parsed"}},
		{HealthFinding: tuikit.HealthFinding{Family: "Coherence", Severity: tuikit.SeverityError, Section: "Git", Title: "missing fragment"}},
		{HealthFinding: tuikit.HealthFinding{Family: "Permissions", Severity: tuikit.SeverityCritical, Section: "SSH", Title: "private key exposed"}},
	}
	filtered := suppressParseErrorFindings(findings)
	if len(filtered) != 2 {
		t.Fatalf("filtered findings = %d, want 2: %+v", len(filtered), filtered)
	}
	for _, finding := range filtered {
		if finding.Section == "Git" && finding.Family != "Files" {
			t.Fatalf("Git ordinary finding survived parse suppression: %+v", finding)
		}
	}
}
