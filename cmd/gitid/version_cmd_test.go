package main

import (
	"bytes"
	"encoding/json"
	"runtime"
	"strings"
	"testing"
)

// TestVersionCmdMatchesVersionFlagOutput asserts `gitid version` (no flags)
// prints the identical line `gitid --version` prints — both surfaces share
// versionString(), so they can never disagree.
func TestVersionCmdMatchesVersionFlagOutput(t *testing.T) {
	var buf bytes.Buffer
	cmd := newRootCmd()
	cmd.SetOut(&buf)
	cmd.SetArgs([]string{"version"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute(version): %v", err)
	}
	want := versionString() + "\n"
	if got := buf.String(); got != want {
		t.Fatalf("gitid version output = %q, want %q", got, want)
	}
}

// TestVersionCmdJSON asserts `gitid version --json` emits a single-line JSON
// document with schema "gitid.version/v1" whose fields agree with
// versionString()'s composed stamp (same version.Resolve() call, same
// runtime.GOOS/GOARCH platform string).
func TestVersionCmdJSON(t *testing.T) {
	var buf bytes.Buffer
	cmd := newRootCmd()
	cmd.SetOut(&buf)
	cmd.SetArgs([]string{"version", "--json"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute(version --json): %v", err)
	}

	var doc versionDocument
	if err := json.Unmarshal(buf.Bytes(), &doc); err != nil {
		t.Fatalf("json.Unmarshal(%q): %v", buf.String(), err)
	}
	if doc.Schema != versionSchema {
		t.Errorf("schema = %q, want %q", doc.Schema, versionSchema)
	}
	wantPlatform := runtime.GOOS + "/" + runtime.GOARCH
	if doc.Platform != wantPlatform {
		t.Errorf("platform = %q, want %q", doc.Platform, wantPlatform)
	}
	// versionString() composes "<version> (<commit>, <date>, <platform>)" —
	// each JSON field must appear verbatim inside that composed stamp so the
	// two surfaces can never silently disagree.
	composed := versionString()
	for name, val := range map[string]string{
		"version":    doc.Version,
		"commit":     doc.Commit,
		"build_date": doc.BuildDate,
	} {
		if !strings.Contains(composed, val) {
			t.Errorf("%s = %q not found in --version's composed stamp %q", name, val, composed)
		}
	}
}

// TestVersionCmdNoArgs asserts `gitid version` rejects positional arguments.
func TestVersionCmdNoArgs(t *testing.T) {
	var buf bytes.Buffer
	cmd := newRootCmd()
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"version", "extra-arg"})
	if err := cmd.Execute(); err == nil {
		t.Fatal("Execute(version extra-arg) should have errored (cobra.NoArgs)")
	}
}
