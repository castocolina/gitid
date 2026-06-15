package gitconfig

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ── Task 1: renderer tests ──────────────────────────────────────────────────

// TestRenderBaselineBlock_Full verifies that the full default config produces
// the byte-identical render specified in RESEARCH Example 1 — fixed section
// order, tab-prefixed keys, no trailing newline, and no [user] section.
func TestRenderBaselineBlock(t *testing.T) {
	t.Run("full default config equals RESEARCH Example 1", func(t *testing.T) {
		cfg := DefaultBaselineConfig()
		got := RenderBaselineBlock(cfg)

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
		first := RenderBaselineBlock(cfg)
		second := RenderBaselineBlock(cfg)
		if first != second {
			t.Error("RenderBaselineBlock: two calls with same cfg produced different output (map iteration?)")
		}
	})

	t.Run("Tier-2 off omits autocrlf/pager/extra-colors/diff/merge/init/alias", func(t *testing.T) {
		cfg := BaselineConfig{
			// Only Tier-1: no Tier-2 flags set.
		}
		got := RenderBaselineBlock(cfg)

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
		got := RenderBaselineBlock(cfg)

		if strings.Contains(got, "[merge]") {
			t.Errorf("expected [merge] section to be absent when MergeConflictStyle is empty, got:\n%s", got)
		}
		if strings.Contains(got, "conflictstyle") {
			t.Errorf("expected conflictstyle key to be absent when MergeConflictStyle is empty, got:\n%s", got)
		}
	})

	t.Run("no [user] section ever emitted (D-04b)", func(t *testing.T) {
		cfg := DefaultBaselineConfig()
		got := RenderBaselineBlock(cfg)
		if strings.Contains(got, "[user]") {
			t.Errorf("RenderBaselineBlock must never emit [user] section, got:\n%s", got)
		}
	})

	t.Run("no trailing newline (TrimRight contract)", func(t *testing.T) {
		cfg := DefaultBaselineConfig()
		got := RenderBaselineBlock(cfg)
		if strings.HasSuffix(got, "\n") {
			t.Errorf("RenderBaselineBlock must not end with a newline, last char: %q", got[len(got)-1])
		}
	})
}

// TestRenderURLRewritesBlock verifies the url-rewrites block render against
// RESEARCH Example 2 and the injection guard.
func TestRenderURLRewritesBlock(t *testing.T) {
	t.Run("default big-three equals RESEARCH Example 2", func(t *testing.T) {
		rewrites := DefaultURLRewrites()
		got := RenderURLRewritesBlock(rewrites)

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
		got := RenderURLRewritesBlock(nil)
		if got != "" {
			t.Errorf("RenderURLRewritesBlock(nil) = %q, want empty string", got)
		}
	})

	t.Run("byte-stable across two calls", func(t *testing.T) {
		rewrites := DefaultURLRewrites()
		first := RenderURLRewritesBlock(rewrites)
		second := RenderURLRewritesBlock(rewrites)
		if first != second {
			t.Error("RenderURLRewritesBlock: two calls produced different output")
		}
	})

	t.Run("newline in HTTPSPrefix panics (injection guard)", func(t *testing.T) {
		defer func() {
			if r := recover(); r == nil {
				t.Error("expected panic for newline-injected HTTPSPrefix, got none")
			}
		}()
		_ = RenderURLRewritesBlock([]URLRewrite{
			{HTTPSPrefix: "https://github.com/\nevil=injected", SSHPrefix: "git@github.com:"},
		})
	})

	t.Run("newline in SSHPrefix panics (injection guard)", func(t *testing.T) {
		defer func() {
			if r := recover(); r == nil {
				t.Error("expected panic for newline-injected SSHPrefix, got none")
			}
		}()
		_ = RenderURLRewritesBlock([]URLRewrite{
			{HTTPSPrefix: "https://github.com/", SSHPrefix: "git@github.com:\nevil=injected"},
		})
	})
}

// TestRenderGitignoreBlock verifies the gitignore block render against
// RESEARCH Example 4.
func TestRenderGitignoreBlock(t *testing.T) {
	t.Run("default 13 patterns in fixed order", func(t *testing.T) {
		patterns := DefaultGitignorePatterns()
		got := RenderGitignoreBlock(patterns)

		// RESEARCH Example 4 — exact byte string.
		want := ".DS_Store\n" +
			"Thumbs.db\n" +
			"*.log\n" +
			"*.bak\n" +
			"*.tmp\n" +
			"*.swp\n" +
			"*.swo\n" +
			".idea/\n" +
			".vscode/\n" +
			"node_modules/\n" +
			"__pycache__/\n" +
			"*.pyc\n" +
			".env"

		if got != want {
			t.Errorf("RenderGitignoreBlock output mismatch.\ngot:\n%s\nwant:\n%s", got, want)
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

// ── Task 3 (Plan 03): ScanConflicts + RemoveURLRewritesBlock tests ──────────

// TestScanConflicts verifies that ScanConflicts strips all managed blocks first
// (RESEARCH C2 / Pitfall C) before comparing against the baseline key set.
// Tests use real temp gitconfig files; git is a guaranteed runtime dependency.
func TestScanConflicts(t *testing.T) {
	t.Run("managed-block keys not reported — only user keys overlap", func(t *testing.T) {
		// A gitconfig that has BOTH a user core.ignorecase=true (outside any
		// managed block) AND a gitid managed baseline-include block that contains
		// core.ignorecase=false. ScanConflicts must return exactly ONE conflict
		// for the user key, not two (Pitfall C: managed-block keys must be stripped).
		dir := t.TempDir()
		gitconfigPath := filepath.Join(dir, ".gitconfig")

		content := "# BEGIN gitid managed: baseline-include\n" +
			"[include]\n\tpath = ~/.gitconfig.d/00-baseline\n" +
			"# END gitid managed: baseline-include\n" +
			"[core]\n\tignorecase = true\n"
		if err := os.WriteFile(gitconfigPath, []byte(content), 0o644); err != nil { //nolint:gosec // test path
			t.Fatalf("writing temp gitconfig: %v", err)
		}

		baselineKeys := BaselineKeySet()
		conflicts, err := ScanConflicts(gitconfigPath, baselineKeys)
		if err != nil {
			t.Fatalf("ScanConflicts: %v", err)
		}

		// Should find exactly one conflict: core.ignorecase (user=true, baseline=false)
		if len(conflicts) != 1 {
			t.Fatalf("expected 1 conflict, got %d: %v", len(conflicts), conflicts)
		}
		c := conflicts[0]
		if c.Key != "core.ignorecase" {
			t.Errorf("expected key=core.ignorecase, got %q", c.Key)
		}
		if c.UserValue != "true" {
			t.Errorf("expected user value=true, got %q", c.UserValue)
		}
		if c.Winner != "user" {
			t.Errorf("expected winner=user, got %q", c.Winner)
		}
	})

	t.Run("excludesfile conflict reported (Pitfall E)", func(t *testing.T) {
		dir := t.TempDir()
		gitconfigPath := filepath.Join(dir, ".gitconfig")

		content := "[core]\n\texcludesfile = ~/.gitignore\n"
		if err := os.WriteFile(gitconfigPath, []byte(content), 0o644); err != nil { //nolint:gosec // test path
			t.Fatalf("writing temp gitconfig: %v", err)
		}

		baselineKeys := BaselineKeySet()
		conflicts, err := ScanConflicts(gitconfigPath, baselineKeys)
		if err != nil {
			t.Fatalf("ScanConflicts: %v", err)
		}

		// Must find core.excludesfile conflict (Pitfall E)
		var found bool
		for _, c := range conflicts {
			if c.Key == "core.excludesfile" {
				found = true
				if c.Winner != "user" {
					t.Errorf("expected winner=user for excludesfile conflict, got %q", c.Winner)
				}
				if c.BaselineValue != "~/.gitignore_global" {
					t.Errorf("expected baseline value=~/.gitignore_global, got %q", c.BaselineValue)
				}
			}
		}
		if !found {
			t.Error("expected core.excludesfile conflict, none found")
		}
	})

	t.Run("no-overlap: non-baseline user keys return empty slice", func(t *testing.T) {
		dir := t.TempDir()
		gitconfigPath := filepath.Join(dir, ".gitconfig")

		content := "[user]\n\tname = Test User\n\temail = test@example.com\n"
		if err := os.WriteFile(gitconfigPath, []byte(content), 0o644); err != nil { //nolint:gosec // test path
			t.Fatalf("writing temp gitconfig: %v", err)
		}

		baselineKeys := BaselineKeySet()
		conflicts, err := ScanConflicts(gitconfigPath, baselineKeys)
		if err != nil {
			t.Fatalf("ScanConflicts: %v", err)
		}
		if len(conflicts) != 0 {
			t.Errorf("expected 0 conflicts for non-baseline user keys, got %d: %v", len(conflicts), conflicts)
		}
	})

	t.Run("missing file returns empty slice and nil error", func(t *testing.T) {
		dir := t.TempDir()
		gitconfigPath := filepath.Join(dir, "nonexistent.gitconfig")

		baselineKeys := BaselineKeySet()
		conflicts, err := ScanConflicts(gitconfigPath, baselineKeys)
		if err != nil {
			t.Fatalf("ScanConflicts on missing file: expected nil error, got %v", err)
		}
		if len(conflicts) != 0 {
			t.Errorf("expected empty conflicts for missing file, got %d: %v", len(conflicts), conflicts)
		}
	})
}

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
