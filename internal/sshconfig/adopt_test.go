package sshconfig

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/castocolina/gitid/internal/filewriter"
)

// mustMkdir creates dir (and parents) or fails the test.
func mustMkdir(t *testing.T, dir string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatalf("mkdir %s: %v", dir, err)
	}
}

// mustWriteFile writes content to path at mode 0600 or fails the test.
func mustWriteFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("writing %s: %v", path, err)
	}
}

// mustWriteSentinelFile writes a gitid-managed sentinel block for name to
// path, using filewriter.ReplaceBlock as the single source of truth for the
// sentinel format (rather than hand-rolling BEGIN/END markers).
func mustWriteSentinelFile(t *testing.T, path, name string) {
	t.Helper()
	body := filewriter.ReplaceBlock(nil, name, "Host "+name+".github.com\n  Hostname ssh.github.com\n")
	if err := os.WriteFile(path, body, 0o600); err != nil {
		t.Fatalf("writing sentinel fixture %s: %v", path, err)
	}
}

// --- DetectInclude ---

// TestDetectIncludeNoDirectiveReturnsEmpty proves a config with no Include
// directive returns an empty (nil) result, not an error.
func TestDetectIncludeNoDirectiveReturnsEmpty(t *testing.T) {
	home := t.TempDir()
	sshDir := filepath.Join(home, ".ssh")
	mustMkdir(t, sshDir)
	configPath := filepath.Join(sshDir, "config")
	mustWriteFile(t, configPath, "Host example\n  Hostname example.com\n")

	got, err := DetectInclude(configPath)
	if err != nil {
		t.Fatalf("DetectInclude: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("expected empty result, got %v", got)
	}
}

// TestDetectIncludeMissingFileReturnsEmpty proves a missing config file
// returns an empty result, not an error (first-run case).
func TestDetectIncludeMissingFileReturnsEmpty(t *testing.T) {
	home := t.TempDir()
	configPath := filepath.Join(home, ".ssh", "config") // never created

	got, err := DetectInclude(configPath)
	if err != nil {
		t.Fatalf("DetectInclude on missing file: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("expected empty result, got %v", got)
	}
}

// TestDetectIncludeMultipleDirectivesOrderPreserved proves EVERY Include
// directive is returned in file order (first-match-wins order preserved),
// not just the first.
func TestDetectIncludeMultipleDirectivesOrderPreserved(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	sshDir := filepath.Join(home, ".ssh")
	mustMkdir(t, sshDir)
	configPath := filepath.Join(sshDir, "config")
	mustWriteFile(t, configPath,
		"Include ~/.ssh/config.d/first.config\n"+
			"Host inline\n  Hostname example.com\n"+
			"Include ~/.ssh/config.d/second.config\n")

	got, err := DetectInclude(configPath)
	if err != nil {
		t.Fatalf("DetectInclude: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 Include directives, got %d: %v", len(got), got)
	}
	if got[0].Raw != "~/.ssh/config.d/first.config" {
		t.Errorf("directive[0].Raw = %q, want first.config", got[0].Raw)
	}
	if got[1].Raw != "~/.ssh/config.d/second.config" {
		t.Errorf("directive[1].Raw = %q, want second.config", got[1].Raw)
	}
}

// TestDetectIncludeQuotedPathExpandedConsistently proves a quoted Include
// path is parsed (quotes stripped) and expanded the same way as an unquoted
// tilde path.
func TestDetectIncludeQuotedPathExpandedConsistently(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	sshDir := filepath.Join(home, ".ssh")
	mustMkdir(t, sshDir)
	configPath := filepath.Join(sshDir, "config")
	mustWriteFile(t, configPath, `Include "~/.ssh/config.d/gitid.config"`+"\n")

	got, err := DetectInclude(configPath)
	if err != nil {
		t.Fatalf("DetectInclude: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 Include directive, got %d: %v", len(got), got)
	}
	if !got[0].Quoted {
		t.Error("expected Quoted = true for a double-quoted Include path")
	}
	if got[0].Raw != "~/.ssh/config.d/gitid.config" {
		t.Errorf("Raw = %q, want quotes stripped", got[0].Raw)
	}
	wantExpanded := filepath.Join(sshDir, "config.d", "gitid.config")
	if got[0].Expanded != wantExpanded {
		t.Errorf("Expanded = %q, want %q", got[0].Expanded, wantExpanded)
	}
}

// TestDetectIncludeAbsolutePathExpandsToItself proves an absolute Include
// path is returned unchanged as Expanded.
func TestDetectIncludeAbsolutePathExpandsToItself(t *testing.T) {
	home := t.TempDir()
	sshDir := filepath.Join(home, ".ssh")
	mustMkdir(t, sshDir)
	configPath := filepath.Join(sshDir, "config")
	absTarget := filepath.Join(sshDir, "config.d", "abs.config")
	mustWriteFile(t, configPath, "Include "+absTarget+"\n")

	got, err := DetectInclude(configPath)
	if err != nil {
		t.Fatalf("DetectInclude: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 Include directive, got %d: %v", len(got), got)
	}
	if got[0].Expanded != absTarget {
		t.Errorf("Expanded = %q, want %q", got[0].Expanded, absTarget)
	}
}

// --- Adopt ---

// TestAdopt covers the selection-rule matrix (STORE-02) via real, filesystem-
// backed fixtures under a hermetic t.TempDir() HOME (Pitfall 5).
func TestAdopt(t *testing.T) {
	type setupResult struct {
		configPath string
		method     AdoptMethod
		chosenPath string
	}

	cases := []struct {
		name       string
		setup      func(t *testing.T, sshDir string) setupResult
		wantTarget func(sshDir string) string // "" means AdoptCreateConfigD fallback
		wantMethod AdoptMethod
	}{
		{
			name: "config.d glob resolving to exactly one gitid file is adopted",
			setup: func(t *testing.T, sshDir string) setupResult {
				configDir := filepath.Join(sshDir, "config.d")
				mustMkdir(t, configDir)
				mustWriteSentinelFile(t, filepath.Join(configDir, "gitid.config"), "personal")
				configPath := filepath.Join(sshDir, "config")
				mustWriteFile(t, configPath, "Include ~/.ssh/config.d/*.config\n")
				return setupResult{configPath: configPath, method: AdoptSentinelBearing}
			},
			wantTarget: func(sshDir string) string { return filepath.Join(sshDir, "config.d", "gitid.config") },
			wantMethod: AdoptSentinelBearing,
		},
		{
			name: "quoted single-path Include resolving to a gitid file is adopted",
			setup: func(t *testing.T, sshDir string) setupResult {
				configDir := filepath.Join(sshDir, "config.d")
				mustMkdir(t, configDir)
				mustWriteSentinelFile(t, filepath.Join(configDir, "gitid.config"), "personal")
				configPath := filepath.Join(sshDir, "config")
				mustWriteFile(t, configPath, `Include "~/.ssh/config.d/gitid.config"`+"\n")
				return setupResult{configPath: configPath, method: AdoptSentinelBearing}
			},
			wantTarget: func(sshDir string) string { return filepath.Join(sshDir, "config.d", "gitid.config") },
			wantMethod: AdoptSentinelBearing,
		},
		{
			name: "broad glob resolving to exactly one non-gitid file is REJECTED",
			setup: func(t *testing.T, sshDir string) setupResult {
				mustWriteFile(t, filepath.Join(sshDir, "other.conf"), "Host other\n  Hostname other.example.com\n")
				configPath := filepath.Join(sshDir, "config")
				mustWriteFile(t, configPath, "Include ~/.ssh/*.conf\n")
				return setupResult{configPath: configPath, method: AdoptSentinelBearing}
			},
			wantTarget: func(string) string { return "" },
			wantMethod: AdoptCreateConfigD,
		},
		{
			name: "glob resolving to multiple gitid-owned files is ambiguous and REJECTED",
			setup: func(t *testing.T, sshDir string) setupResult {
				configDir := filepath.Join(sshDir, "config.d")
				mustMkdir(t, configDir)
				mustWriteSentinelFile(t, filepath.Join(configDir, "personal.config"), "personal")
				mustWriteSentinelFile(t, filepath.Join(configDir, "work.config"), "work")
				configPath := filepath.Join(sshDir, "config")
				mustWriteFile(t, configPath, "Include ~/.ssh/config.d/*.config\n")
				return setupResult{configPath: configPath, method: AdoptSentinelBearing}
			},
			wantTarget: func(string) string { return "" },
			wantMethod: AdoptCreateConfigD,
		},
		{
			name: "bare-relative path (no ~/.ssh prefix) is REJECTED at the boundary",
			setup: func(t *testing.T, sshDir string) setupResult {
				configDir := filepath.Join(sshDir, "config.d")
				mustMkdir(t, configDir)
				mustWriteSentinelFile(t, filepath.Join(configDir, "gitid.config"), "personal")
				configPath := filepath.Join(sshDir, "config")
				mustWriteFile(t, configPath, "Include config.d/gitid.config\n")
				return setupResult{configPath: configPath, method: AdoptSentinelBearing}
			},
			wantTarget: func(string) string { return "" },
			wantMethod: AdoptCreateConfigD,
		},
		{
			name: "non-~/.ssh-relative tilde path is REJECTED at the boundary",
			setup: func(t *testing.T, sshDir string) setupResult {
				docsDir := filepath.Join(sshDir, "..", "Documents")
				mustMkdir(t, docsDir)
				mustWriteSentinelFile(t, filepath.Join(docsDir, "foo.config"), "personal")
				configPath := filepath.Join(sshDir, "config")
				mustWriteFile(t, configPath, "Include ~/Documents/foo.config\n")
				return setupResult{configPath: configPath, method: AdoptSentinelBearing}
			},
			wantTarget: func(string) string { return "" },
			wantMethod: AdoptCreateConfigD,
		},
		{
			name: "symlinked target is REJECTED even when sentinel-bearing",
			setup: func(t *testing.T, sshDir string) setupResult {
				configDir := filepath.Join(sshDir, "config.d")
				mustMkdir(t, configDir)
				realTarget := filepath.Join(sshDir, "real-target.config")
				mustWriteSentinelFile(t, realTarget, "personal")
				symlinkPath := filepath.Join(configDir, "gitid.config")
				if err := os.Symlink(realTarget, symlinkPath); err != nil {
					t.Fatalf("creating symlink fixture: %v", err)
				}
				configPath := filepath.Join(sshDir, "config")
				mustWriteFile(t, configPath, "Include ~/.ssh/config.d/*.config\n")
				return setupResult{configPath: configPath, method: AdoptSentinelBearing}
			},
			wantTarget: func(string) string { return "" },
			wantMethod: AdoptCreateConfigD,
		},
		{
			name: "caller-chosen path is adopted despite an ambiguous, non-gitid glob",
			setup: func(t *testing.T, sshDir string) setupResult {
				chosen := filepath.Join(sshDir, "chosen.conf")
				mustWriteFile(t, chosen, "Host chosen\n  Hostname chosen.example.com\n")
				mustWriteFile(t, filepath.Join(sshDir, "other.conf"), "Host other\n  Hostname other.example.com\n")
				configPath := filepath.Join(sshDir, "config")
				mustWriteFile(t, configPath, "Include ~/.ssh/*.conf\n")
				return setupResult{configPath: configPath, method: AdoptCallerChosen, chosenPath: chosen}
			},
			wantTarget: func(sshDir string) string { return filepath.Join(sshDir, "chosen.conf") },
			wantMethod: AdoptCallerChosen,
		},
		{
			name: "AdoptCreateConfigD bypasses detection entirely",
			setup: func(t *testing.T, sshDir string) setupResult {
				configDir := filepath.Join(sshDir, "config.d")
				mustMkdir(t, configDir)
				mustWriteSentinelFile(t, filepath.Join(configDir, "gitid.config"), "personal")
				configPath := filepath.Join(sshDir, "config")
				mustWriteFile(t, configPath, "Include ~/.ssh/config.d/*.config\n")
				return setupResult{configPath: configPath, method: AdoptCreateConfigD}
			},
			wantTarget: func(string) string { return "" },
			wantMethod: AdoptCreateConfigD,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			home := t.TempDir()
			t.Setenv("HOME", home)
			sshDir := filepath.Join(home, ".ssh")
			mustMkdir(t, sshDir)

			sr := c.setup(t, sshDir)

			result, err := Adopt(sr.configPath, sr.method, sr.chosenPath, RealAdoptDeps())
			if err != nil {
				t.Fatalf("Adopt: %v", err)
			}

			wantTarget := c.wantTarget(sshDir)
			if result.TargetPath != wantTarget {
				t.Errorf("TargetPath = %q, want %q", result.TargetPath, wantTarget)
			}
			if result.Method != c.wantMethod {
				t.Errorf("Method = %v, want %v", result.Method, c.wantMethod)
			}
		})
	}
}
