package gitconfig

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/castocolina/gitid/internal/filewriter"
)

// ── Task 1: renderer tests ──────────────────────────────────────────────────

// TestRenderBaselineBlock_Full verifies that the full default config produces
// the byte-identical render specified in RESEARCH Example 1 — fixed section
// order, tab-prefixed keys, no trailing newline, and no [user] section.
func TestRenderBaselineBlock(t *testing.T) {
	t.Run("full default config equals RESEARCH Example 1", func(t *testing.T) {
		cfg := DefaultBaselineConfig()
		got, err := RenderBaselineBlock(cfg)
		if err != nil {
			t.Fatalf("RenderBaselineBlock: unexpected error: %v", err)
		}

		// RESEARCH Example 1 — exact byte string (tabs, not spaces).
		want := "[core]\n" +
			"\tignorecase = false\n" +
			"\texcludesfile = ~/.gitignore_global\n" +
			"\tautocrlf = input\n" +
			"\tpager = less -FRX\n" +
			"[push]\n" +
			"\tautoSetupRemote = true\n" +
			"[pull]\n" +
			"\trebase = true\n" +
			"[fetch]\n" +
			"\tprune = true\n" +
			"[color]\n" +
			"\tui = auto\n" +
			"\tbranch = auto\n" +
			"\tdiff = auto\n" +
			"\tstatus = auto\n" +
			"[diff]\n" +
			"\tcolorMoved = zebra\n" +
			"[merge]\n" +
			"\tconflictstyle = zdiff3\n" +
			"[init]\n" +
			"\tdefaultBranch = main\n" +
			"[alias]\n" +
			"\tst = status\n" +
			"\tco = checkout\n" +
			"\tbr = branch\n" +
			"\tci = commit\n" +
			"\tdf = diff\n" +
			"\tlg = log --graph --pretty=format:'%Cred%h%Creset -%C(yellow)%d%Creset %s %Cgreen(%cr) %C(bold blue)<%an>%Creset' --abbrev-commit\n" +
			"\tunstage = reset HEAD --\n" +
			"\tlast = log -1 HEAD"

		if got != want {
			t.Errorf("RenderBaselineBlock output mismatch.\ngot:\n%s\nwant:\n%s", got, want)
		}
	})

	t.Run("byte-stable across two calls (no map iteration)", func(t *testing.T) {
		cfg := DefaultBaselineConfig()
		first, err := RenderBaselineBlock(cfg)
		if err != nil {
			t.Fatalf("first RenderBaselineBlock: %v", err)
		}
		second, err := RenderBaselineBlock(cfg)
		if err != nil {
			t.Fatalf("second RenderBaselineBlock: %v", err)
		}
		if first != second {
			t.Error("RenderBaselineBlock: two calls with same cfg produced different output (map iteration?)")
		}
	})

	t.Run("Tier-2 off omits autocrlf/pager/extra-colors/diff/merge/init/alias", func(t *testing.T) {
		cfg := BaselineConfig{
			// Only Tier-1: no Tier-2 flags set.
		}
		got, err := RenderBaselineBlock(cfg)
		if err != nil {
			t.Fatalf("RenderBaselineBlock: %v", err)
		}

		// Tier-1 keys must be present
		for _, mustHave := range []string{
			"[core]",
			"\tignorecase = false",
			"\texcludesfile = ~/.gitignore_global",
			"[push]",
			"\tautoSetupRemote = true",
			"[pull]",
			"\trebase = true",
			"[fetch]",
			"\tprune = true",
			"[color]",
			"\tui = auto",
		} {
			if !strings.Contains(got, mustHave) {
				t.Errorf("Tier-1 key missing from output: %q\noutput:\n%s", mustHave, got)
			}
		}

		// Tier-2 keys must be absent
		for _, mustAbsent := range []string{
			"autocrlf",
			"pager",
			"\tbranch = auto",
			"\tdiff = auto",
			"\tstatus = auto",
			"[diff]",
			"colorMoved",
			"[merge]",
			"conflictstyle",
			"[init]",
			"defaultBranch",
			"[alias]",
		} {
			if strings.Contains(got, mustAbsent) {
				t.Errorf("Tier-2 key present when it should be absent: %q\noutput:\n%s", mustAbsent, got)
			}
		}
	})

	t.Run("zdiff3 gate: empty MergeConflictStyle omits [merge] section", func(t *testing.T) {
		cfg := DefaultBaselineConfig()
		cfg.MergeConflictStyle = ""
		got, err := RenderBaselineBlock(cfg)
		if err != nil {
			t.Fatalf("RenderBaselineBlock: %v", err)
		}

		if strings.Contains(got, "[merge]") {
			t.Errorf("expected [merge] section to be absent when MergeConflictStyle is empty, got:\n%s", got)
		}
		if strings.Contains(got, "conflictstyle") {
			t.Errorf("expected conflictstyle key to be absent when MergeConflictStyle is empty, got:\n%s", got)
		}
	})

	t.Run("no [user] section ever emitted (D-04b)", func(t *testing.T) {
		cfg := DefaultBaselineConfig()
		got, err := RenderBaselineBlock(cfg)
		if err != nil {
			t.Fatalf("RenderBaselineBlock: %v", err)
		}
		if strings.Contains(got, "[user]") {
			t.Errorf("RenderBaselineBlock must never emit [user] section, got:\n%s", got)
		}
	})

	t.Run("no trailing newline (TrimRight contract)", func(t *testing.T) {
		cfg := DefaultBaselineConfig()
		got, err := RenderBaselineBlock(cfg)
		if err != nil {
			t.Fatalf("RenderBaselineBlock: %v", err)
		}
		if strings.HasSuffix(got, "\n") {
			t.Errorf("RenderBaselineBlock must not end with a newline, last char: %q", got[len(got)-1])
		}
	})

	t.Run("invalid Pager returns error not panic (WR-03)", func(t *testing.T) {
		cfg := DefaultBaselineConfig()
		cfg.Pager = "less\nevil=injected"
		_, err := RenderBaselineBlock(cfg)
		if err == nil {
			t.Error("WR-03: expected error for newline-injected Pager, got nil")
		}
	})
}

// TestRenderURLRewritesBlock verifies the url-rewrites block render against
// RESEARCH Example 2 and the injection guard.
func TestRenderURLRewritesBlock(t *testing.T) {
	t.Run("default big-three equals RESEARCH Example 2", func(t *testing.T) {
		rewrites := DefaultURLRewrites()
		got, err := RenderURLRewritesBlock(rewrites)
		if err != nil {
			t.Fatalf("RenderURLRewritesBlock: unexpected error: %v", err)
		}

		// RESEARCH Example 2 — exact byte string.
		want := "[url \"git@github.com:\"]\n" +
			"\tinsteadOf = https://github.com/\n" +
			"[url \"git@gitlab.com:\"]\n" +
			"\tinsteadOf = https://gitlab.com/\n" +
			"[url \"git@bitbucket.org:\"]\n" +
			"\tinsteadOf = https://bitbucket.org/"

		if got != want {
			t.Errorf("RenderURLRewritesBlock output mismatch.\ngot:\n%s\nwant:\n%s", got, want)
		}
	})

	t.Run("empty slice returns empty string", func(t *testing.T) {
		got, err := RenderURLRewritesBlock(nil)
		if err != nil {
			t.Fatalf("RenderURLRewritesBlock(nil): unexpected error: %v", err)
		}
		if got != "" {
			t.Errorf("RenderURLRewritesBlock(nil) = %q, want empty string", got)
		}
	})

	t.Run("byte-stable across two calls", func(t *testing.T) {
		rewrites := DefaultURLRewrites()
		first, err := RenderURLRewritesBlock(rewrites)
		if err != nil {
			t.Fatalf("first RenderURLRewritesBlock: %v", err)
		}
		second, err := RenderURLRewritesBlock(rewrites)
		if err != nil {
			t.Fatalf("second RenderURLRewritesBlock: %v", err)
		}
		if first != second {
			t.Error("RenderURLRewritesBlock: two calls produced different output")
		}
	})

	t.Run("newline in HTTPSPrefix returns error not panic (WR-03, injection guard)", func(t *testing.T) {
		_, err := RenderURLRewritesBlock([]URLRewrite{
			{HTTPSPrefix: "https://github.com/\nevil=injected", SSHPrefix: "git@github.com:"},
		})
		if err == nil {
			t.Error("WR-03: expected error for newline-injected HTTPSPrefix, got nil")
		}
	})

	t.Run("newline in SSHPrefix returns error not panic (WR-03, injection guard)", func(t *testing.T) {
		_, err := RenderURLRewritesBlock([]URLRewrite{
			{HTTPSPrefix: "https://github.com/", SSHPrefix: "git@github.com:\nevil=injected"},
		})
		if err == nil {
			t.Error("WR-03: expected error for newline-injected SSHPrefix, got nil")
		}
	})
}

// TestRenderGitignoreBlock verifies the gitignore block render against
// the extended catalog with comment headers. The extended catalog is a strict
// superset of the old 13 patterns, with comment headers and additional patterns
// for Python venv, Node, and tooling caches.
func TestRenderGitignoreBlock(t *testing.T) {
	t.Run("extended catalog renders with comment headers", func(t *testing.T) {
		patterns := DefaultGitignorePatterns()
		got := RenderGitignoreBlock(patterns)

		// The extended catalog includes comment headers for readability.
		// Check for markers of all major sections.
		sections := []string{
			"# OS artifacts",
			"# Editors and IDEs",
			"# Logs, temp and scratch",
			"# Environment files",
			"# Python",
			"# Node",
			"# Tooling caches",
		}
		for _, section := range sections {
			if !strings.Contains(got, section) {
				t.Errorf("expected section %q in rendered output", section)
			}
		}

		// All old 13 entries must be present (strict superset).
		oldThirteen := []string{
			".DS_Store", "Thumbs.db", "*.log", "*.bak", "*.tmp", "*.swp", "*.swo",
			".idea/", ".vscode/", "node_modules/", "__pycache__/", "*.pyc", ".env",
		}
		for _, entry := range oldThirteen {
			if !strings.Contains(got, entry) {
				t.Errorf("old entry %q missing from rendered output", entry)
			}
		}

		// New GIGN-01 entries must be present.
		newEntries := []string{
			"desktop.ini", ".env.*", "!.env.example", ".venv/", "venv/",
			"tmp/", ".tmp/", ".direnv/", ".pytest_cache/", ".mypy_cache/", ".ruff_cache/",
		}
		for _, entry := range newEntries {
			if !strings.Contains(got, entry) {
				t.Errorf("new entry %q missing from rendered output", entry)
			}
		}
	})

	t.Run("byte-stable across two calls", func(t *testing.T) {
		patterns := DefaultGitignorePatterns()
		first := RenderGitignoreBlock(patterns)
		second := RenderGitignoreBlock(patterns)
		if first != second {
			t.Error("RenderGitignoreBlock: two calls produced different output")
		}
	})

	t.Run("empty slice returns empty string", func(t *testing.T) {
		got := RenderGitignoreBlock(nil)
		if got != "" {
			t.Errorf("RenderGitignoreBlock(nil) = %q, want empty string", got)
		}
	})
}

// ── Task 2: writer tests ────────────────────────────────────────────────────

// TestWriteBaselineFile_Idempotent verifies that writing the baseline file
// twice produces byte-identical content (SC-1).
func TestWriteBaselineFile_Idempotent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".gitconfig.d", "00-baseline")

	cfg := DefaultBaselineConfig()
	rewrites := DefaultURLRewrites()

	_, err := WriteBaselineFile(path, cfg, rewrites)
	if err != nil {
		t.Fatalf("first WriteBaselineFile: %v", err)
	}
	first, err := os.ReadFile(path) //nolint:gosec // test path
	if err != nil {
		t.Fatalf("reading after first write: %v", err)
	}

	_, err = WriteBaselineFile(path, cfg, rewrites)
	if err != nil {
		t.Fatalf("second WriteBaselineFile: %v", err)
	}
	second, err := os.ReadFile(path) //nolint:gosec // test path
	if err != nil {
		t.Fatalf("reading after second write: %v", err)
	}

	if !bytes.Equal(first, second) {
		t.Errorf("WriteBaselineFile is not idempotent: file content differs between first and second write.\nfirst:\n%s\nsecond:\n%s", first, second)
	}

	// Verify all six SC-2 locked patterns appear (via baseline block's excludesfile key).
	content := string(second)
	if !strings.Contains(content, "excludesfile = ~/.gitignore_global") {
		t.Error("baseline block missing excludesfile key")
	}
}

// TestWriteBaselineFile_PreservesForeign verifies that foreign git settings
// outside the managed blocks are preserved verbatim after WriteBaselineFile.
func TestWriteBaselineFile_PreservesForeign(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".gitconfig.d", "00-baseline")

	// Seed the file with foreign content.
	foreignContent := "[user]\n\tname = Foreign User\n\temail = foreign@example.com\n"
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(path, []byte(foreignContent), 0o644); err != nil { //nolint:gosec // test path
		t.Fatalf("seeding foreign content: %v", err)
	}

	cfg := DefaultBaselineConfig()
	rewrites := DefaultURLRewrites()

	_, err := WriteBaselineFile(path, cfg, rewrites)
	if err != nil {
		t.Fatalf("WriteBaselineFile: %v", err)
	}

	content, err := os.ReadFile(path) //nolint:gosec // test path
	if err != nil {
		t.Fatalf("reading file: %v", err)
	}

	// Foreign content must be present verbatim.
	if !strings.Contains(string(content), "[user]") {
		t.Error("foreign [user] section was removed; expected it to be preserved")
	}
	if !strings.Contains(string(content), "foreign@example.com") {
		t.Error("foreign email was removed; expected it to be preserved")
	}

	// Managed baseline block must be present.
	if !strings.Contains(string(content), "# BEGIN gitid managed: baseline") {
		t.Error("baseline managed block sentinel missing")
	}
	// Managed url-rewrites block must be present.
	if !strings.Contains(string(content), "# BEGIN gitid managed: url-rewrites") {
		t.Error("url-rewrites managed block sentinel missing")
	}
}

// TestWriteGlobalGitignore_Idempotent verifies that writing the global gitignore
// twice produces byte-identical content (SC-2).
func TestWriteGlobalGitignore_Idempotent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".gitignore_global")

	patterns := DefaultGitignorePatterns()

	_, err := WriteGlobalGitignore(path, patterns)
	if err != nil {
		t.Fatalf("first WriteGlobalGitignore: %v", err)
	}
	first, err := os.ReadFile(path) //nolint:gosec // test path
	if err != nil {
		t.Fatalf("reading after first write: %v", err)
	}

	_, err = WriteGlobalGitignore(path, patterns)
	if err != nil {
		t.Fatalf("second WriteGlobalGitignore: %v", err)
	}
	second, err := os.ReadFile(path) //nolint:gosec // test path
	if err != nil {
		t.Fatalf("reading after second write: %v", err)
	}

	if !bytes.Equal(first, second) {
		t.Errorf("WriteGlobalGitignore is not idempotent.\nfirst:\n%s\nsecond:\n%s", first, second)
	}

	// All six SC-2-locked patterns must be in the managed block.
	content := string(second)
	for _, p := range []string{".DS_Store", "Thumbs.db", "*.log", "*.bak", "*.tmp", "*.swp"} {
		if !strings.Contains(content, p) {
			t.Errorf("SC-2 locked pattern missing: %q", p)
		}
	}
}

// TestWriteGlobalGitignore_PreservesForeign verifies that foreign user ignore
// lines outside the managed block are preserved verbatim.
func TestWriteGlobalGitignore_PreservesForeign(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".gitignore_global")

	// Seed with a foreign pattern the user added manually.
	foreignContent := "# My personal ignores\n*.secret\nbuild/\n"
	if err := os.WriteFile(path, []byte(foreignContent), 0o644); err != nil { //nolint:gosec // test path
		t.Fatalf("seeding foreign content: %v", err)
	}

	patterns := DefaultGitignorePatterns()
	_, err := WriteGlobalGitignore(path, patterns)
	if err != nil {
		t.Fatalf("WriteGlobalGitignore: %v", err)
	}

	content, err := os.ReadFile(path) //nolint:gosec // test path
	if err != nil {
		t.Fatalf("reading file: %v", err)
	}

	// Foreign content must be present verbatim.
	if !strings.Contains(string(content), "*.secret") {
		t.Error("foreign *.secret pattern was removed; expected it to be preserved")
	}
	if !strings.Contains(string(content), "build/") {
		t.Error("foreign build/ pattern was removed; expected it to be preserved")
	}

	// Managed block must be present.
	if !strings.Contains(string(content), "# BEGIN gitid managed: gitignore") {
		t.Error("gitignore managed block sentinel missing")
	}
}

// ── Task 3 (Plan 03): RemoveURLRewritesBlock tests ────────────────────────

// TestRemoveURLRewritesBlock verifies that RemoveURLRewritesBlock removes only
// the url-rewrites block, leaving the baseline block and foreign content intact.
func TestRemoveURLRewritesBlock(t *testing.T) {
	t.Run("removes url-rewrites block, preserves baseline block and foreign content", func(t *testing.T) {
		dir := t.TempDir()
		baselineFilePath := filepath.Join(dir, "00-baseline")

		cfg := DefaultBaselineConfig()
		rewrites := DefaultURLRewrites()
		_, err := WriteBaselineFile(baselineFilePath, cfg, rewrites)
		if err != nil {
			t.Fatalf("WriteBaselineFile: %v", err)
		}

		// Verify both blocks exist before removal.
		content, _ := os.ReadFile(baselineFilePath) //nolint:gosec // test path
		if !strings.Contains(string(content), "# BEGIN gitid managed: url-rewrites") {
			t.Fatal("url-rewrites block missing before test")
		}
		if !strings.Contains(string(content), "# BEGIN gitid managed: baseline") {
			t.Fatal("baseline block missing before test")
		}

		_, err = RemoveURLRewritesBlock(baselineFilePath)
		if err != nil {
			t.Fatalf("RemoveURLRewritesBlock: %v", err)
		}

		after, err := os.ReadFile(baselineFilePath) //nolint:gosec // test path
		if err != nil {
			t.Fatalf("reading after removal: %v", err)
		}
		s := string(after)

		// url-rewrites block must be gone.
		if strings.Contains(s, "# BEGIN gitid managed: url-rewrites") {
			t.Error("url-rewrites block still present after RemoveURLRewritesBlock")
		}
		if strings.Contains(s, "insteadOf") {
			t.Error("insteadOf entries still present after RemoveURLRewritesBlock")
		}

		// baseline block must still be present (only url-rewrites removed).
		if !strings.Contains(s, "# BEGIN gitid managed: baseline") {
			t.Error("baseline block was removed; expected it to be preserved")
		}
		if !strings.Contains(s, "ignorecase = false") {
			t.Error("baseline block content missing; expected it to be preserved")
		}
	})

	t.Run("re-run is a no-op (idempotent)", func(t *testing.T) {
		dir := t.TempDir()
		baselineFilePath := filepath.Join(dir, "00-baseline")

		cfg := DefaultBaselineConfig()
		_, err := WriteBaselineFile(baselineFilePath, cfg, nil) // no rewrites
		if err != nil {
			t.Fatalf("WriteBaselineFile: %v", err)
		}

		first, _ := os.ReadFile(baselineFilePath) //nolint:gosec // test path

		_, err = RemoveURLRewritesBlock(baselineFilePath)
		if err != nil {
			t.Fatalf("RemoveURLRewritesBlock (no-op): %v", err)
		}

		second, _ := os.ReadFile(baselineFilePath) //nolint:gosec // test path
		if !bytes.Equal(first, second) {
			t.Error("RemoveURLRewritesBlock not idempotent: file changed when url-rewrites block was absent")
		}
	})
}

// ── Task 4 (Plan 03): ReadBaselineState tests ────────────────────────────────

// TestReadBaselineState verifies that ReadBaselineState reconstructs the managed
// baseline state from the three disk files with no sidecar DB (IDENT-07 model).
func TestReadBaselineState(t *testing.T) {
	t.Run("installed: all three files present with managed blocks", func(t *testing.T) {
		dir := t.TempDir()
		gitconfigPath := filepath.Join(dir, ".gitconfig")
		baselineFilePath := filepath.Join(dir, ".gitconfig.d", "00-baseline")
		gitignorePath := filepath.Join(dir, ".gitignore_global")

		// Write the gitconfig include block.
		_, err := WriteBaselineInclude(gitconfigPath, baselineFilePath)
		if err != nil {
			t.Fatalf("WriteBaselineInclude: %v", err)
		}

		// Write the baseline file with all defaults.
		cfg := DefaultBaselineConfig()
		rewrites := DefaultURLRewrites()
		_, err = WriteBaselineFile(baselineFilePath, cfg, rewrites)
		if err != nil {
			t.Fatalf("WriteBaselineFile: %v", err)
		}

		// Write the global gitignore.
		patterns := DefaultGitignorePatterns()
		_, err = WriteGlobalGitignore(gitignorePath, patterns)
		if err != nil {
			t.Fatalf("WriteGlobalGitignore: %v", err)
		}

		state, err := ReadBaselineState(gitconfigPath, baselineFilePath, gitignorePath)
		if err != nil {
			t.Fatalf("ReadBaselineState: %v", err)
		}

		if !state.Installed {
			t.Error("expected Installed=true when all three files have managed blocks")
		}
		if state.Incomplete {
			t.Error("expected Incomplete=false when fully installed")
		}
		if len(state.Missing) != 0 {
			t.Errorf("expected no missing artifacts, got: %v", state.Missing)
		}
		if len(state.URLRewrites) != 3 {
			t.Errorf("expected 3 url-rewrite mappings, got %d: %v", len(state.URLRewrites), state.URLRewrites)
		}
		if len(state.GitignorePatterns) < 6 {
			t.Errorf("expected at least 6 gitignore patterns (SC-2), got %d", len(state.GitignorePatterns))
		}
		if len(state.BaselineKeys) == 0 {
			t.Error("expected non-empty baseline keys map")
		}
	})

	t.Run("not-installed: no managed blocks anywhere", func(t *testing.T) {
		dir := t.TempDir()
		// Paths exist but have no managed blocks.
		gitconfigPath := filepath.Join(dir, ".gitconfig")
		baselineFilePath := filepath.Join(dir, ".gitconfig.d", "00-baseline")
		gitignorePath := filepath.Join(dir, ".gitignore_global")

		state, err := ReadBaselineState(gitconfigPath, baselineFilePath, gitignorePath)
		if err != nil {
			t.Fatalf("ReadBaselineState on empty dir: %v", err)
		}
		if state.Installed {
			t.Error("expected Installed=false when no managed blocks exist")
		}
	})

	t.Run("incomplete: include block present but 00-baseline missing", func(t *testing.T) {
		dir := t.TempDir()
		gitconfigPath := filepath.Join(dir, ".gitconfig")
		baselineFilePath := filepath.Join(dir, ".gitconfig.d", "00-baseline") // not created
		gitignorePath := filepath.Join(dir, ".gitignore_global")

		// Only write the include block — no baseline file.
		_, err := WriteBaselineInclude(gitconfigPath, baselineFilePath)
		if err != nil {
			t.Fatalf("WriteBaselineInclude: %v", err)
		}

		state, err := ReadBaselineState(gitconfigPath, baselineFilePath, gitignorePath)
		if err != nil {
			t.Fatalf("ReadBaselineState: %v", err)
		}

		if state.Installed {
			t.Error("expected Installed=false for incomplete state")
		}
		if !state.Incomplete {
			t.Error("expected Incomplete=true when include block present but baseline file missing")
		}
		if len(state.Missing) == 0 {
			t.Error("expected Missing to be non-empty for incomplete state")
		}
	})

	t.Run("missing files do not error (first-run case)", func(t *testing.T) {
		dir := t.TempDir()
		gitconfigPath := filepath.Join(dir, "nonexistent", ".gitconfig")
		baselineFilePath := filepath.Join(dir, "nonexistent", "00-baseline")
		gitignorePath := filepath.Join(dir, "nonexistent", ".gitignore_global")

		state, err := ReadBaselineState(gitconfigPath, baselineFilePath, gitignorePath)
		if err != nil {
			t.Fatalf("ReadBaselineState on missing files: expected nil error, got %v", err)
		}
		if state.Installed {
			t.Error("expected Installed=false for all-missing-files case")
		}
	})
}

// ── Review-fix regression tests ─────────────────────────────────────────────

// TestParseGitconfigBlockBody_WR04 verifies that parseGitconfigBlockBody
// preserves values that contain " = " (WR-04 regression).
func TestParseGitconfigBlockBody_WR04(t *testing.T) {
	t.Run("alias value containing ' = ' is preserved verbatim", func(t *testing.T) {
		// An alias whose value contains " = " must not be split at the inner " = ".
		body := "[alias]\n\tfoo = !f() { x = y; }; f\n"
		got := parseGitconfigBlockBody(body)
		want := "!f() { x = y; }; f"
		if got["alias.foo"] != want {
			t.Errorf("parseGitconfigBlockBody: alias.foo = %q, want %q", got["alias.foo"], want)
		}
	})

	t.Run("indented line starting with '[' is NOT treated as a section header", func(t *testing.T) {
		// A tab-indented line that starts with '[' (e.g. a value beginning with a
		// bracket) must not be misidentified as a section header.
		body := "[core]\n\tpager = less -FRX\n"
		got := parseGitconfigBlockBody(body)
		if _, ok := got["core.pager"]; !ok {
			t.Error("expected core.pager to be present")
		}
	})

	t.Run("round-trip: render then parse produces matching keys", func(t *testing.T) {
		cfg := DefaultBaselineConfig()
		rendered, err := RenderBaselineBlock(cfg)
		if err != nil {
			t.Fatalf("RenderBaselineBlock: %v", err)
		}
		parsed := parseGitconfigBlockBody(rendered)
		// Spot-check a few keys that must survive the round-trip.
		for _, key := range []string{"core.ignorecase", "pull.rebase", "alias.lg"} {
			if _, ok := parsed[key]; !ok {
				t.Errorf("round-trip: key %q missing from parsed output", key)
			}
		}
		// alias.lg must contain the full format string (not truncated at first ' = ').
		if v := parsed["alias.lg"]; !strings.Contains(v, "--graph") {
			t.Errorf("round-trip: alias.lg value appears truncated: %q", v)
		}
	})
}

// TestWriteBaselineInclude_WR01 verifies that WriteBaselineInclude uses the
// baselineFilePath parameter to derive the include path, not a hardcoded literal
// (WR-01 regression).
func TestWriteBaselineInclude_WR01(t *testing.T) {
	t.Run("custom baseline path is honoured in include body", func(t *testing.T) {
		dir := t.TempDir()
		gitconfigPath := filepath.Join(dir, ".gitconfig")
		customBaseline := "~/.gitconfig.d/custom-baseline"

		_, err := WriteBaselineInclude(gitconfigPath, customBaseline)
		if err != nil {
			t.Fatalf("WriteBaselineInclude: %v", err)
		}

		content, err := os.ReadFile(gitconfigPath) //nolint:gosec // test path
		if err != nil {
			t.Fatalf("reading gitconfig: %v", err)
		}
		if !strings.Contains(string(content), "path = "+customBaseline) {
			t.Errorf("WR-01: include body does not contain custom path %q; got:\n%s", customBaseline, content)
		}
	})
}

// TestWriteBaselineInclude verifies that the include block is placed at the TOP
// of ~/.gitconfig (floor model) on first write, and updated in-place on second.
func TestWriteBaselineInclude(t *testing.T) {
	dir := t.TempDir()
	gitconfigPath := filepath.Join(dir, ".gitconfig")
	baselineFilePath := "~/.gitconfig.d/00-baseline" // literal ~ as per RESEARCH Q2

	t.Run("fresh gitconfig: block placed at TOP, existing content preserved", func(t *testing.T) {
		// Seed with existing [user] content.
		existing := "[user]\n\tname = Test User\n\temail = test@example.com\n"
		if err := os.WriteFile(gitconfigPath, []byte(existing), 0o644); err != nil { //nolint:gosec // test path
			t.Fatalf("seeding gitconfig: %v", err)
		}

		_, err := WriteBaselineInclude(gitconfigPath, baselineFilePath)
		if err != nil {
			t.Fatalf("WriteBaselineInclude: %v", err)
		}

		content, err := os.ReadFile(gitconfigPath) //nolint:gosec // test path
		if err != nil {
			t.Fatalf("reading gitconfig: %v", err)
		}

		s := string(content)

		// Block must be present with correct include body (RESEARCH Example 3).
		if !strings.Contains(s, "# BEGIN gitid managed: baseline-include") {
			t.Error("baseline-include sentinel missing")
		}
		if !strings.Contains(s, "[include]") {
			t.Error("[include] section missing")
		}
		if !strings.Contains(s, "\tpath = ~/.gitconfig.d/00-baseline") {
			t.Error("include path line missing or wrong")
		}

		// The baseline-include block must appear BEFORE the existing [user] content.
		beginIdx := strings.Index(s, "# BEGIN gitid managed: baseline-include")
		userIdx := strings.Index(s, "[user]")
		if beginIdx == -1 || userIdx == -1 {
			t.Fatal("expected both BEGIN sentinel and [user] section to be present")
		}
		if beginIdx >= userIdx {
			t.Errorf("baseline-include block (%d) is not before existing [user] section (%d) — floor model violated", beginIdx, userIdx)
		}
	})

	t.Run("second write: block updated in-place, not duplicated", func(t *testing.T) {
		content, err := os.ReadFile(gitconfigPath) //nolint:gosec // test path
		if err != nil {
			t.Fatalf("reading gitconfig before second write: %v", err)
		}
		firstBeginPos := strings.Index(string(content), "# BEGIN gitid managed: baseline-include")

		_, err = WriteBaselineInclude(gitconfigPath, baselineFilePath)
		if err != nil {
			t.Fatalf("second WriteBaselineInclude: %v", err)
		}

		content2, err := os.ReadFile(gitconfigPath) //nolint:gosec // test path
		if err != nil {
			t.Fatalf("reading gitconfig after second write: %v", err)
		}

		s2 := string(content2)

		// No duplicate sentinels.
		count := strings.Count(s2, "# BEGIN gitid managed: baseline-include")
		if count != 1 {
			t.Errorf("expected exactly 1 baseline-include BEGIN sentinel, got %d", count)
		}

		// Block position must not have moved (in-place update preserves floor).
		secondBeginPos := strings.Index(s2, "# BEGIN gitid managed: baseline-include")
		if firstBeginPos != secondBeginPos {
			t.Errorf("baseline-include block moved on second write: first=%d second=%d", firstBeginPos, secondBeginPos)
		}
	})
}

// ── Plan 09.2-01: NormalizeGitignoreLines / ComposeGlobalGitignore / InspectManagedBlockFile ──

func TestNormalizeGitignoreLines(t *testing.T) {
	t.Run("CRLF trailing spaces and trailing blank run", func(t *testing.T) {
		in := ".DS_Store  \r\n\r\n*.log   \r\n\r\n\r\n"
		got, err := NormalizeGitignoreLines(in)
		if err != nil {
			t.Fatalf("NormalizeGitignoreLines: %v", err)
		}
		want := []string{".DS_Store", "", "*.log"}
		if len(got) != len(want) {
			t.Fatalf("got %q, want %q", got, want)
		}
		for i := range want {
			if got[i] != want[i] {
				t.Errorf("line %d = %q, want %q", i, got[i], want[i])
			}
		}
	})

	t.Run("sentinel prefix is refused with line number", func(t *testing.T) {
		in := ".DS_Store\n# BEGIN gitid managed: nested\n*.log\n"
		_, err := NormalizeGitignoreLines(in)
		if err == nil {
			t.Fatal("expected error for a nested BEGIN sentinel")
		}
		msg := err.Error()
		if !strings.Contains(msg, "2") {
			t.Errorf("error must name the offending line number, got %q", msg)
		}
		if !strings.Contains(strings.ToLower(msg), "begin") && !strings.Contains(strings.ToLower(msg), "sentinel") && !strings.Contains(strings.ToLower(msg), "marker") {
			t.Errorf("error must name the sentinel, got %q", msg)
		}
	})

	t.Run("round trip DefaultGitignorePatterns", func(t *testing.T) {
		rendered := RenderGitignoreBlock(DefaultGitignorePatterns())
		got, err := NormalizeGitignoreLines(rendered)
		if err != nil {
			t.Fatalf("NormalizeGitignoreLines: %v", err)
		}
		want := DefaultGitignorePatterns()
		if len(got) != len(want) {
			t.Fatalf("got %q, want %q", got, want)
		}
		for i := range want {
			if got[i] != want[i] {
				t.Errorf("line %d = %q, want %q", i, got[i], want[i])
			}
		}
	})
}

func TestComposeGlobalGitignore(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".gitignore_global")
	existing := []byte("# foreign before\n*.secret\n")
	if err := os.WriteFile(path, existing, 0o644); err != nil { //nolint:gosec // test path
		t.Fatalf("seeding: %v", err)
	}
	patterns := DefaultGitignorePatterns()
	composed := ComposeGlobalGitignore(existing, patterns)
	if _, err := WriteGlobalGitignore(path, patterns); err != nil {
		t.Fatalf("WriteGlobalGitignore: %v", err)
	}
	written, err := os.ReadFile(path) //nolint:gosec // test path
	if err != nil {
		t.Fatalf("reading written file: %v", err)
	}
	if !bytes.Equal(composed, written) {
		t.Errorf("ComposeGlobalGitignore diverged from WriteGlobalGitignore.\ncomposed:\n%s\nwritten:\n%s", composed, written)
	}
}

func managedBlock(name, body string) string {
	return filewriter.BeginPrefix + name + "\n" + body + "\n" + filewriter.EndPrefix + name + "\n"
}

func TestInspectManagedBlockFile(t *testing.T) {
	t.Run("empty content is healthy with no managed block", func(t *testing.T) {
		shape, err := InspectManagedBlockFile(nil, "gitignore")
		if err != nil {
			t.Fatalf("empty content: %v", err)
		}
		if shape.Managed {
			t.Error("empty content must report Managed=false")
		}
		if shape.Body != "" {
			t.Errorf("empty content body = %q, want empty", shape.Body)
		}
	})

	t.Run("one complete block surrounded by foreign lines", func(t *testing.T) {
		content := []byte("# foreign before\n" + managedBlock("gitignore", ".DS_Store\n*.log") + "# foreign after\n")
		shape, err := InspectManagedBlockFile(content, "gitignore")
		if err != nil {
			t.Fatalf("healthy file: %v", err)
		}
		if !shape.Managed {
			t.Error("one complete block must report Managed=true")
		}
		if !strings.Contains(shape.Body, ".DS_Store") || !strings.Contains(shape.Body, "*.log") {
			t.Errorf("body = %q, want the managed-block body", shape.Body)
		}
		if strings.Contains(shape.Body, "foreign") {
			t.Errorf("body must not include foreign lines, got %q", shape.Body)
		}
	})

	t.Run("five malformed shapes", func(t *testing.T) {
		cases := []struct {
			name    string
			content string
		}{
			{"orphan BEGIN", filewriter.BeginPrefix + "gitignore\n.DS_Store\n"},
			{"standalone END", filewriter.EndPrefix + "gitignore\n"},
			{"nested BEGIN", managedBlock("gitignore", filewriter.BeginPrefix+"gitignore\n.DS_Store")},
			{"mismatched END name", filewriter.BeginPrefix + "gitignore\n.DS_Store\n" + filewriter.EndPrefix + "other\n"},
			{"duplicate complete blocks", managedBlock("gitignore", ".DS_Store") + managedBlock("gitignore", "*.log")},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				_, err := InspectManagedBlockFile([]byte(tc.content), "gitignore")
				if err == nil {
					t.Fatalf("expected error for %s", tc.name)
				}
				msg := err.Error()
				if !strings.ContainsAny(msg, "0123456789") {
					t.Errorf("error must name an offending line number, got %q", msg)
				}
				if !strings.Contains(strings.ToLower(msg), "repair") && !strings.Contains(strings.ToLower(msg), "hand") {
					t.Errorf("error must say the file must be repaired by hand, got %q", msg)
				}
			})
		}
	})

	t.Run("duplicate baseline blocks are refused", func(t *testing.T) {
		content := managedBlock("baseline", "\texcludesfile = ~/.gitignore_global") +
			managedBlock("baseline", "\texcludesfile = ~/other")
		_, err := InspectManagedBlockFile([]byte(content), "baseline")
		if err == nil {
			t.Fatal("two complete baseline blocks must be refused")
		}
		if !strings.Contains(strings.ToLower(err.Error()), "two") && !strings.Contains(strings.ToLower(err.Error()), "duplicate") {
			t.Errorf("error must name the duplicate, got %q", err)
		}
	})

	t.Run("mixed names are healthy for each requested name", func(t *testing.T) {
		content := []byte(managedBlock("gitignore", ".DS_Store") + managedBlock("baseline", "[core]\n\tignorecase = false"))
		gign, err := InspectManagedBlockFile(content, "gitignore")
		if err != nil {
			t.Fatalf("gitignore: %v", err)
		}
		if !gign.Managed {
			t.Error("gitignore half must be healthy")
		}
		base, err := InspectManagedBlockFile(content, "baseline")
		if err != nil {
			t.Fatalf("baseline: %v", err)
		}
		if !base.Managed {
			t.Error("baseline half must be healthy")
		}
	})
}

func TestInspectGitignoreFile(t *testing.T) {
	healthy := []byte("# foreign\n" + managedBlock("gitignore", ".DS_Store") + "# after\n")
	want, err := InspectManagedBlockFile(healthy, "gitignore")
	if err != nil {
		t.Fatalf("InspectManagedBlockFile: %v", err)
	}
	got, err := InspectGitignoreFile(healthy)
	if err != nil {
		t.Fatalf("InspectGitignoreFile: %v", err)
	}
	if got != want {
		t.Errorf("InspectGitignoreFile = %+v, want %+v (must be a one-line wrapper)", got, want)
	}

	malformed := []byte(filewriter.BeginPrefix + "gitignore\n.DS_Store\n")
	_, wantErr := InspectManagedBlockFile(malformed, "gitignore")
	_, gotErr := InspectGitignoreFile(malformed)
	if wantErr == nil || gotErr == nil {
		t.Fatal("both wrappers must refuse an orphan BEGIN")
	}
	if wantErr.Error() != gotErr.Error() {
		t.Errorf("wrapper error = %q, want %q", gotErr, wantErr)
	}
}

// TestInspectManagedBlockFile_NestedOtherNameIsParserBehaviorRecord documents
// (does not change) the shared scanner: a complete url-rewrites pair nested
// inside a complete baseline block is HEALTHY for "baseline" because
// listBlocksWith tracks one open block regardless of name and only closes on
// a name-matching END. ReplaceBlock rewrites that same span, so read and
// write agree — this is why the inspector stays requested-name-scoped.
func TestInspectManagedBlockFile_NestedOtherNameIsParserBehaviorRecord(t *testing.T) {
	nested := filewriter.BeginPrefix + "url-rewrites\n" +
		`[url "git@github.com:"]` + "\n\tinsteadOf = https://github.com/\n" +
		filewriter.EndPrefix + "url-rewrites"
	body := "[core]\n\tignorecase = false\n" + nested + "\n"
	content := []byte("# foreign before\n" + managedBlock("baseline", body) + "# foreign after\n")

	shape, err := InspectManagedBlockFile(content, "baseline")
	if err != nil {
		t.Fatalf("nested other-name pair must be healthy for baseline: %v", err)
	}
	if !shape.Managed {
		t.Fatal("expected exactly one healthy baseline block")
	}
	if !strings.Contains(shape.Body, filewriter.BeginPrefix+"url-rewrites") {
		t.Errorf("body must carry the nested marker lines verbatim, got %q", shape.Body)
	}

	rewritten := filewriter.ReplaceBlock(content, "baseline", "[core]\n\tignorecase = true")
	if !strings.Contains(string(rewritten), "# foreign before") || !strings.Contains(string(rewritten), "# foreign after") {
		t.Error("ReplaceBlock must leave foreign content outside the reported span")
	}
	if strings.Contains(string(rewritten), "url-rewrites") {
		t.Error("ReplaceBlock must rewrite the same span the inspector reported, replacing the nested pair with the rest of the body")
	}
	if strings.Contains(string(rewritten), "ignorecase = false") {
		t.Error("the original body must be replaced")
	}
}

// ── Task 1.09.2: Extended catalog and comment-free entry view tests ─────────

func TestDefaultGitignorePatterns_ExtendedCatalog(t *testing.T) {
	t.Run("contains all previous thirteen entries", func(t *testing.T) {
		entries := DefaultGitignoreEntries()

		oldThirteen := []string{
			".DS_Store", "Thumbs.db", "*.log", "*.bak", "*.tmp", "*.swp", "*.swo",
			".idea/", ".vscode/", "node_modules/", "__pycache__/", "*.pyc", ".env",
		}
		for _, want := range oldThirteen {
			found := false
			for _, e := range entries {
				if e == want {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("previous entry %q missing from extended catalog", want)
			}
		}
	})

	t.Run("contains each pattern GIGN-01 names", func(t *testing.T) {
		entries := DefaultGitignoreEntries()

		gign01Patterns := []string{
			".env", ".env.*", "!.env.example", ".venv/", "venv/",
		}
		for _, want := range gign01Patterns {
			found := false
			for _, e := range entries {
				if e == want {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("GIGN-01 required pattern %q missing from catalog", want)
			}
		}
	})

	t.Run("negation entry appears strictly after wildcard env entry", func(t *testing.T) {
		entries := DefaultGitignoreEntries()

		envIdx := -1
		negIdx := -1
		for i, e := range entries {
			if e == ".env.*" {
				envIdx = i
			}
			if e == "!.env.example" {
				negIdx = i
			}
		}

		if envIdx == -1 || negIdx == -1 {
			t.Fatalf(".env.* or !.env.example missing: envIdx=%d, negIdx=%d", envIdx, negIdx)
		}
		if envIdx >= negIdx {
			t.Errorf("negation !.env.example must come AFTER .env.*, got indices %d and %d", envIdx, negIdx)
		}
	})

	t.Run("patterns contains at least one comment-header line", func(t *testing.T) {
		patterns := DefaultGitignorePatterns()

		hasComment := false
		for _, p := range patterns {
			trimmed := strings.TrimSpace(p)
			if strings.HasPrefix(trimmed, "#") {
				hasComment = true
				break
			}
		}
		if !hasComment {
			t.Error("DefaultGitignorePatterns must contain at least one comment-header line")
		}
	})

	t.Run("entries contains no comment-header lines", func(t *testing.T) {
		entries := DefaultGitignoreEntries()

		for _, e := range entries {
			trimmed := strings.TrimSpace(e)
			if strings.HasPrefix(trimmed, "#") {
				t.Errorf("DefaultGitignoreEntries must not contain comment, found %q", e)
			}
		}
	})

	t.Run("entries equals ordered comment-and-blank-free filtering of patterns", func(t *testing.T) {
		patterns := DefaultGitignorePatterns()
		entries := DefaultGitignoreEntries()

		// Manually filter patterns to remove comments and blanks.
		var filtered []string
		for _, p := range patterns {
			trimmed := strings.TrimSpace(p)
			if trimmed != "" && !strings.HasPrefix(trimmed, "#") {
				filtered = append(filtered, trimmed)
			}
		}

		if len(filtered) != len(entries) {
			t.Fatalf("length mismatch: filtered=%d, entries=%d", len(filtered), len(entries))
		}
		for i, want := range filtered {
			if entries[i] != want {
				t.Errorf("index %d: entries[%d]=%q, want %q", i, i, entries[i], want)
			}
		}
	})

	t.Run("round trip through read path produces entries from pattern block", func(t *testing.T) {
		patterns := DefaultGitignorePatterns()
		rendered := RenderGitignoreBlock(patterns)

		// Read it back through the same parsing parseGitignoreBlockBody uses.
		readBack := parseGitignoreBlockBody(rendered)

		entries := DefaultGitignoreEntries()
		if len(readBack) != len(entries) {
			t.Fatalf("round trip length mismatch: readBack=%d, entries=%d", len(readBack), len(entries))
		}
		for i, want := range entries {
			if readBack[i] != want {
				t.Errorf("round trip index %d: readBack[%d]=%q, want %q", i, i, readBack[i], want)
			}
		}
	})

	t.Run("WriteGlobalGitignore with extended defaults is idempotent", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, ".gitignore_global")

		patterns := DefaultGitignorePatterns()

		_, err := WriteGlobalGitignore(path, patterns)
		if err != nil {
			t.Fatalf("first WriteGlobalGitignore: %v", err)
		}
		first, err := os.ReadFile(path) //nolint:gosec // test path
		if err != nil {
			t.Fatalf("reading after first write: %v", err)
		}

		_, err = WriteGlobalGitignore(path, patterns)
		if err != nil {
			t.Fatalf("second WriteGlobalGitignore: %v", err)
		}
		second, err := os.ReadFile(path) //nolint:gosec // test path
		if err != nil {
			t.Fatalf("reading after second write: %v", err)
		}

		if !bytes.Equal(first, second) {
			t.Errorf("WriteGlobalGitignore not idempotent with extended defaults.\nfirst:\n%s\nsecond:\n%s", first, second)
		}
	})

	t.Run("ComposeGlobalGitignore produces bytes identical to WriteGlobalGitignore", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, ".gitignore_global")

		existing := []byte("# user comment\n*.mypattern\n")
		if err := os.WriteFile(path, existing, 0o644); err != nil { //nolint:gosec // test path
			t.Fatalf("seeding: %v", err)
		}

		patterns := DefaultGitignorePatterns()
		composed := ComposeGlobalGitignore(existing, patterns)

		_, err := WriteGlobalGitignore(path, patterns)
		if err != nil {
			t.Fatalf("WriteGlobalGitignore: %v", err)
		}

		written, err := os.ReadFile(path) //nolint:gosec // test path
		if err != nil {
			t.Fatalf("reading written file: %v", err)
		}

		if !bytes.Equal(composed, written) {
			t.Errorf("ComposeGlobalGitignore diverged from WriteGlobalGitignore.\ncomposed:\n%s\nwritten:\n%s", composed, written)
		}
	})

	t.Run("preserves foreign content with extended defaults", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, ".gitignore_global")

		foreignContent := "# My patterns\n*.custom\nbuild/\n"
		if err := os.WriteFile(path, []byte(foreignContent), 0o644); err != nil { //nolint:gosec // test path
			t.Fatalf("seeding: %v", err)
		}

		patterns := DefaultGitignorePatterns()
		_, err := WriteGlobalGitignore(path, patterns)
		if err != nil {
			t.Fatalf("WriteGlobalGitignore: %v", err)
		}

		content, err := os.ReadFile(path) //nolint:gosec // test path
		if err != nil {
			t.Fatalf("reading: %v", err)
		}

		s := string(content)
		if !strings.Contains(s, "*.custom") {
			t.Error("foreign *.custom pattern was removed")
		}
		if !strings.Contains(s, "build/") {
			t.Error("foreign build/ pattern was removed")
		}
		if !strings.Contains(s, "# BEGIN gitid managed: gitignore") {
			t.Error("managed block sentinel missing")
		}
	})
}
