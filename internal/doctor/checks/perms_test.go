package checks_test

import (
	"os"
	"testing"
	"time"

	"github.com/castocolina/gitid/internal/doctor"
	"github.com/castocolina/gitid/internal/doctor/checks"
)

// fakeFileInfo is a minimal os.FileInfo implementation for perms tests.
// It allows precise control over Mode() without touching the real filesystem.
type fakeFileInfo struct {
	name  string
	mode  os.FileMode
	isDir bool
}

func (f fakeFileInfo) Name() string       { return f.name }
func (f fakeFileInfo) Size() int64        { return 0 }
func (f fakeFileInfo) Mode() os.FileMode  { return f.mode }
func (f fakeFileInfo) ModTime() time.Time { return time.Time{} }
func (f fakeFileInfo) IsDir() bool        { return f.isDir }
func (f fakeFileInfo) Sys() interface{}   { return nil }

// makeMissingStat returns a Stat function that always returns os.ErrNotExist.
func makeMissingStat() func(string) (os.FileInfo, error) {
	return func(_ string) (os.FileInfo, error) {
		return nil, &os.PathError{Op: "stat", Path: "fake", Err: os.ErrNotExist}
	}
}

// TestCheckPermsPass verifies that correct KEY-02 modes (key 0600, dir 0700,
// .pub 0644, config 0600) yield zero findings.
func TestCheckPermsPass(t *testing.T) {
	// Build deps with a Stat function that returns different modes based on path
	// suffix (key → 0600, dir → 0700, .pub → 0644, config → 0600).
	statFn := func(path string) (os.FileInfo, error) {
		switch {
		case len(path) > 4 && path[len(path)-4:] == ".pub":
			return fakeFileInfo{name: path, mode: 0o644}, nil
		case path == "/home/u/.ssh":
			return fakeFileInfo{name: path, mode: 0o700, isDir: true}, nil
		default:
			return fakeFileInfo{name: path, mode: 0o600}, nil
		}
	}
	deps := doctor.Deps{
		Stat:               statFn,
		FixPerm:            func(_ string, _ os.FileMode) error { return nil },
		SSHDir:             "/home/u/.ssh",
		SSHConfigPath:      "/home/u/.ssh/config",
		GitconfigPath:      "/home/u/.gitconfig",
		AllowedSignersPath: "/home/u/.ssh/allowed_signers",
	}
	findings := checks.CheckPermissions(deps)
	if len(findings) != 0 {
		t.Errorf("CheckPermissions with correct modes returned %d findings (want 0): %+v", len(findings), findings)
	}
}

// TestCheckPermsCritical verifies that a private key at 0644 produces exactly
// one critical Permissions finding with the correct suggested fix.
func TestCheckPermsCritical(t *testing.T) {
	const keyPath = "/home/u/.ssh/gitid_work"
	statFn := func(path string) (os.FileInfo, error) {
		if path == keyPath {
			return fakeFileInfo{name: path, mode: 0o644}, nil // wrong: should be 0600
		}
		if path == "/home/u/.ssh" {
			return fakeFileInfo{name: path, mode: 0o700, isDir: true}, nil
		}
		if len(path) > 4 && path[len(path)-4:] == ".pub" {
			return fakeFileInfo{name: path, mode: 0o644}, nil
		}
		return fakeFileInfo{name: path, mode: 0o600}, nil
	}

	deps := doctor.Deps{
		Stat:               statFn,
		FixPerm:            func(_ string, _ os.FileMode) error { return nil },
		SSHDir:             "/home/u/.ssh",
		SSHConfigPath:      "/home/u/.ssh/config",
		GitconfigPath:      "/home/u/.gitconfig",
		AllowedSignersPath: "/home/u/.ssh/allowed_signers",
		KeyPaths:           []string{keyPath},
	}
	findings := checks.CheckPermissions(deps)

	// Filter to findings about the specific key path.
	var keyFindings []doctor.Finding
	for _, f := range findings {
		if f.Family == doctor.FamilyPerms && containsStr(f.Title, keyPath) {
			keyFindings = append(keyFindings, f)
		}
	}

	if len(keyFindings) != 1 {
		t.Fatalf("expected exactly 1 Permissions finding for key at 0644, got %d: %+v", len(keyFindings), findings)
	}
	f := keyFindings[0]
	if f.Severity != doctor.SeverityCritical {
		t.Errorf("severity = %v, want critical", f.Severity)
	}
	if !containsStr(f.SuggestedFix, "chmod 0600") {
		t.Errorf("SuggestedFix = %q, want to contain 'chmod 0600'", f.SuggestedFix)
	}
	if !containsStr(f.SuggestedFix, keyPath) {
		t.Errorf("SuggestedFix = %q, want to contain path %q", f.SuggestedFix, keyPath)
	}
	if f.Fix == nil {
		t.Error("Fix must be non-nil (auto-fixable)")
	}
}

// TestCheckPermsDirError verifies that ~/.ssh at 0755 produces an error-severity
// finding with a chmod 0700 suggested fix.
func TestCheckPermsDirError(t *testing.T) {
	statFn := func(path string) (os.FileInfo, error) {
		if path == "/home/u/.ssh" {
			return fakeFileInfo{name: path, mode: 0o755, isDir: true}, nil // wrong: should be 0700
		}
		if len(path) > 4 && path[len(path)-4:] == ".pub" {
			return fakeFileInfo{name: path, mode: 0o644}, nil
		}
		return fakeFileInfo{name: path, mode: 0o600}, nil
	}
	deps := doctor.Deps{
		Stat:               statFn,
		FixPerm:            func(_ string, _ os.FileMode) error { return nil },
		SSHDir:             "/home/u/.ssh",
		SSHConfigPath:      "/home/u/.ssh/config",
		GitconfigPath:      "/home/u/.gitconfig",
		AllowedSignersPath: "/home/u/.ssh/allowed_signers",
	}
	findings := checks.CheckPermissions(deps)

	var dirFindings []doctor.Finding
	for _, f := range findings {
		if f.Family == doctor.FamilyPerms && containsStr(f.Title, "/home/u/.ssh") {
			dirFindings = append(dirFindings, f)
		}
	}

	if len(dirFindings) == 0 {
		t.Fatal("expected at least 1 Permissions finding for .ssh dir at 0755, got 0")
	}
	f := dirFindings[0]
	if f.Severity != doctor.SeverityError {
		t.Errorf("dir finding severity = %v, want error", f.Severity)
	}
	if !containsStr(f.SuggestedFix, "chmod 0700") {
		t.Errorf("SuggestedFix = %q, want to contain 'chmod 0700'", f.SuggestedFix)
	}
}

// TestCheckPermsPubWarning verifies that a .pub file at 0600 (too restrictive)
// yields a warning-severity finding.
func TestCheckPermsPubWarning(t *testing.T) {
	const pubPath = "/home/u/.ssh/gitid_work.pub"
	statFn := func(path string) (os.FileInfo, error) {
		if path == pubPath {
			return fakeFileInfo{name: path, mode: 0o600}, nil // wrong: should be 0644
		}
		if path == "/home/u/.ssh" {
			return fakeFileInfo{name: path, mode: 0o700, isDir: true}, nil
		}
		return fakeFileInfo{name: path, mode: 0o600}, nil
	}
	deps := doctor.Deps{
		Stat:               statFn,
		FixPerm:            func(_ string, _ os.FileMode) error { return nil },
		SSHDir:             "/home/u/.ssh",
		SSHConfigPath:      "/home/u/.ssh/config",
		GitconfigPath:      "/home/u/.gitconfig",
		AllowedSignersPath: "/home/u/.ssh/allowed_signers",
		PubKeyPaths:        []string{pubPath},
	}
	findings := checks.CheckPermissions(deps)

	var pubFindings []doctor.Finding
	for _, f := range findings {
		if f.Family == doctor.FamilyPerms && containsStr(f.Title, pubPath) {
			pubFindings = append(pubFindings, f)
		}
	}

	if len(pubFindings) == 0 {
		t.Fatal("expected at least 1 Permissions finding for .pub at 0600, got 0")
	}
	f := pubFindings[0]
	if f.Severity != doctor.SeverityWarning {
		t.Errorf(".pub finding severity = %v, want warning", f.Severity)
	}
}

// TestCheckPermsMissingSkipped verifies that os.ErrNotExist paths produce no
// Permissions findings (absent files are coherence's concern, not perms).
func TestCheckPermsMissingSkipped(t *testing.T) {
	deps := doctor.Deps{
		Stat:               makeMissingStat(),
		FixPerm:            func(_ string, _ os.FileMode) error { return nil },
		SSHDir:             "/home/u/.ssh",
		SSHConfigPath:      "/home/u/.ssh/config",
		GitconfigPath:      "/home/u/.gitconfig",
		AllowedSignersPath: "/home/u/.ssh/allowed_signers",
	}
	findings := checks.CheckPermissions(deps)
	if len(findings) != 0 {
		t.Errorf("CheckPermissions with all paths missing returned %d findings (want 0): %+v", len(findings), findings)
	}
}

// TestPermFixerFnCallsInjectedFixPerm verifies that the Fix.Fn closure calls
// deps.FixPerm with the correct KEY-02 target mode and path.
func TestPermFixerFnCallsInjectedFixPerm(t *testing.T) {
	const keyPath = "/home/u/.ssh/gitid_work"
	var calledPath string
	var calledMode os.FileMode

	statFn := func(path string) (os.FileInfo, error) {
		if path == keyPath {
			return fakeFileInfo{name: path, mode: 0o644}, nil // wrong
		}
		if path == "/home/u/.ssh" {
			return fakeFileInfo{name: path, mode: 0o700, isDir: true}, nil
		}
		if len(path) > 4 && path[len(path)-4:] == ".pub" {
			return fakeFileInfo{name: path, mode: 0o644}, nil
		}
		return fakeFileInfo{name: path, mode: 0o600}, nil
	}

	fixPermFn := func(path string, mode os.FileMode) error {
		calledPath = path
		calledMode = mode
		return nil
	}

	deps := doctor.Deps{
		Stat:               statFn,
		FixPerm:            fixPermFn,
		SSHDir:             "/home/u/.ssh",
		SSHConfigPath:      "/home/u/.ssh/config",
		GitconfigPath:      "/home/u/.gitconfig",
		AllowedSignersPath: "/home/u/.ssh/allowed_signers",
		KeyPaths:           []string{keyPath},
	}
	findings := checks.CheckPermissions(deps)

	var keyFinding *doctor.Finding
	for i := range findings {
		if findings[i].Family == doctor.FamilyPerms && containsStr(findings[i].Title, keyPath) {
			keyFinding = &findings[i]
			break
		}
	}

	if keyFinding == nil {
		t.Fatal("expected a perms finding for the 0644 key, got none")
	}
	if keyFinding.Fix == nil {
		t.Fatal("Fix must be non-nil for an auto-fixable perms finding")
	}

	// Invoke the fixer.
	if err := keyFinding.Fix.Fn(); err != nil {
		t.Fatalf("Fix.Fn() returned error: %v", err)
	}
	if calledPath != keyPath {
		t.Errorf("FixPerm called with path %q, want %q", calledPath, keyPath)
	}
	if calledMode != 0o600 {
		t.Errorf("FixPerm called with mode %04o, want 0600", calledMode)
	}
	// Verify the fixer never widens permissions (target is 0600, not ≥ current 0644).
	if calledMode > 0o644 {
		t.Errorf("FixPerm widened permissions: got %04o, current was 0644", calledMode)
	}
}

// containsStr reports whether s contains substr.
func containsStr(s, substr string) bool {
	return len(substr) == 0 || (len(s) >= len(substr) && findSubstr(s, substr))
}

func findSubstr(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
