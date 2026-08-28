package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
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

// TestHealthIdentityFlagScopesAndExcludesGlobal proves D-04's `gitid health
// --identity NAME` filters via Finding.IdentityName to exactly that
// identity's own findings, and — matching the TUI deep-link's documented
// choice (internal/tuikit/health_screen.go's healthModel.findings, which
// scopes via tuikit.FindingsFor) — EXCLUDES global (empty-IdentityName)
// findings entirely from the scoped view.
func TestHealthIdentityFlagScopesAndExcludesGlobal(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	seedDeleteFixture(t, home, "work")
	seedDeleteFixture(t, home, "orphan")
	// Break "orphan"'s own IdentityFile so Coherence Check 1 fires an
	// identity-scoped finding (IdentityName == "orphan").
	if err := os.Remove(filepath.Join(home, ".ssh", "id_ed25519_orphan")); err != nil {
		t.Fatalf("removing orphan key: %v", err)
	}
	if err := os.Remove(filepath.Join(home, ".ssh", "id_ed25519_orphan.pub")); err != nil {
		t.Fatalf("removing orphan pub: %v", err)
	}
	// A global Permissions finding (no IdentityName) — must never appear in
	// EITHER identity's scoped view.
	if err := os.Chmod(filepath.Join(home, ".ssh"), 0o755); err != nil { //nolint:gosec // deliberately loose — asserts the global finding is excluded from scoped views (G301 in test scope)
		t.Fatalf("chmod .ssh: %v", err)
	}

	all := doctorFindings(home)
	var sawGlobalPermsFinding, sawOrphanFinding bool
	for _, f := range all {
		if f.Family == "Permissions" && f.Identity == "" {
			sawGlobalPermsFinding = true
		}
		if f.Identity == "orphan" {
			sawOrphanFinding = true
		}
	}
	if !sawGlobalPermsFinding {
		t.Fatalf("fixture sanity: expected a global Permissions finding, findings: %+v", all)
	}
	if !sawOrphanFinding {
		t.Fatalf("fixture sanity: expected an orphan-scoped finding, findings: %+v", all)
	}

	scoped := findingsForIdentity(all, "orphan")
	if len(scoped) == 0 {
		t.Fatal("findingsForIdentity(orphan) must include orphan's own finding")
	}
	for _, f := range scoped {
		if f.Identity != "orphan" {
			t.Errorf("scoped finding %+v carries the wrong identity", f)
		}
		if f.Family == "Permissions" {
			t.Errorf("scoped view for orphan must EXCLUDE the global Permissions finding: %+v", f)
		}
	}

	scopedWork := findingsForIdentity(all, "work")
	for _, f := range scopedWork {
		if f.Family == "Permissions" {
			t.Errorf("scoped view for work must EXCLUDE the global Permissions finding: %+v", f)
		}
	}

	var output bytes.Buffer
	root := newRootCmd()
	root.SetArgs([]string{"health", "--identity", "orphan"})
	root.SetOut(&output)
	if err := root.Execute(); err != nil {
		t.Fatalf("health --identity orphan: %v", err)
	}
	got := output.String()
	if !strings.Contains(got, "orphan") && !strings.Contains(got, "IdentityFile") {
		t.Errorf("gitid health --identity orphan output missing the scoped finding:\n%s", got)
	}
	if strings.Contains(got, "0755") || strings.Contains(got, ".ssh: ") {
		t.Errorf("gitid health --identity orphan must not print the global Permissions finding:\n%s", got)
	}
}

// TestHealthIdentityCompletionListsRealIdentities proves the --identity flag
// carries real identity-name shell completion, mirroring the project's
// existing identity-noun completion pattern.
func TestHealthIdentityCompletionListsRealIdentities(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	seedDeleteFixture(t, home, "work")

	root := newRootCmd()
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetArgs([]string{"__complete", "health", "--identity", ""})
	if err := root.Execute(); err != nil {
		t.Fatalf("__complete health --identity: %v", err)
	}
	if !strings.Contains(buf.String(), "work") {
		t.Errorf("health --identity completion missing %q:\n%s", "work", buf.String())
	}
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
