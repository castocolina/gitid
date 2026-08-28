package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/castocolina/gitid/internal/tuikit"
)

func TestBaselineGitignoreRealWiring(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	var output bytes.Buffer
	root := newRootCmd()
	root.SetArgs([]string{"health", "--json"})
	root.SetOut(&output)
	if err := root.Execute(); err != nil {
		t.Fatalf("health --json: %v", err)
	}
	var findings []tuikit.DemoFinding
	if err := json.Unmarshal(output.Bytes(), &findings); err != nil {
		t.Fatalf("decoding health output: %v\n%s", err, output.String())
	}
	var found bool
	for _, finding := range findings {
		if finding.Title == "core.excludesfile and global gitignore are not configured" {
			found = true
			if finding.Severity != tuikit.SeverityWarning {
				t.Errorf("Severity = %q, want warning", finding.Severity)
			}
		}
	}
	if !found {
		t.Fatalf("health findings missing gitignore pair: %+v", findings)
	}
}

func TestSetDiffersRealWiring(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	seedInstalledBaseline(t, home)
	gitconfigPath := filepath.Join(home, ".gitconfig")
	if err := os.WriteFile(gitconfigPath, append([]byte(readFile(t, gitconfigPath)), []byte("\n[init]\n\tdefaultBranch = trunk\n")...), 0o600); err != nil {
		t.Fatalf("seeding gitconfig: %v", err)
	}
	if err := fixExcludesfile(gitconfigPath)(filepath.Join(home, ".gitignore_global")); err != nil {
		t.Fatalf("seeding gitignore pair: %v", err)
	}
	var output bytes.Buffer
	root := newRootCmd()
	root.SetArgs([]string{"health", "--json"})
	root.SetOut(&output)
	if err := root.Execute(); err != nil {
		t.Fatalf("health --json: %v", err)
	}
	var findings []tuikit.DemoFinding
	if err := json.Unmarshal(output.Bytes(), &findings); err != nil {
		t.Fatalf("decoding health output: %v", err)
	}
	for _, finding := range findings {
		if finding.Title == "init.defaultBranch: trunk (differs from recommendation)" {
			if finding.Severity != tuikit.SeverityInfo {
				t.Errorf("Severity = %q, want info", finding.Severity)
			}
			return
		}
	}
	t.Fatalf("health findings missing init.defaultBranch override: %+v", findings)
}

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
