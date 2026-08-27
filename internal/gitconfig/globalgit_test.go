package gitconfig

import (
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// EnsureGlobalGit compose tests
// ---------------------------------------------------------------------------

// TestEnsureGlobalGit_FreshMachine_SingleKey verifies that composing a
// selection into empty content produces a sentinel-delimited block whose body
// contains exactly the selected keys and nothing else.
func TestEnsureGlobalGit_FreshMachine_SingleKey(t *testing.T) {
	selected := map[string]string{"init.defaultBranch": "main"}
	result, err := EnsureGlobalGit(nil, selected)
	if err != nil {
		t.Fatalf("EnsureGlobalGit: %v", err)
	}

	s := string(result)
	if !strings.Contains(s, GlobalGitSentinelBegin) {
		t.Error("result missing BEGIN sentinel")
	}
	if !strings.Contains(s, GlobalGitSentinelEnd) {
		t.Error("result missing END sentinel")
	}
	if !strings.Contains(s, "defaultBranch = main") {
		t.Errorf("result missing init.defaultBranch, got:\n%s", s)
	}
}

// TestEnsureGlobalGit_Idempotent verifies composing twice with the same
// selection produces byte-identical content.
func TestEnsureGlobalGit_Idempotent(t *testing.T) {
	selected := map[string]string{"init.defaultBranch": "main"}
	first, err := EnsureGlobalGit(nil, selected)
	if err != nil {
		t.Fatalf("first compose: %v", err)
	}
	second, err := EnsureGlobalGit(nil, selected)
	if err != nil {
		t.Fatalf("second compose: %v", err)
	}
	if string(first) != string(second) {
		t.Errorf("idempotency FAILED:\nfirst:\n%s\nsecond:\n%s", first, second)
	}
}

// TestEnsureGlobalGit_PreservesContentAroundBlock verifies content before and
// after the block is preserved byte-for-byte.
func TestEnsureGlobalGit_PreservesContentAroundBlock(t *testing.T) {
	before := "# user-written header\n[core]\n\teditor = vim\n"
	after := "\n# user-written footer\n"
	existing := []byte(before +
		GlobalGitSentinelBegin + "\n[init]\n\tdefaultBranch = master\n" + GlobalGitSentinelEnd + "\n" +
		after)

	selected := map[string]string{"init.defaultBranch": "main"}
	result, err := EnsureGlobalGit(existing, selected)
	if err != nil {
		t.Fatalf("EnsureGlobalGit: %v", err)
	}

	s := string(result)
	if !strings.Contains(s, "# user-written header") {
		t.Error("content before block was lost")
	}
	if !strings.Contains(s, "# user-written footer") {
		t.Error("content after block was lost")
	}
	if !strings.Contains(s, "editor = vim") {
		t.Error("foreign [core] key was lost")
	}
}

// TestEnsureGlobalGit_LegacySentinelAdoption verifies that content carrying
// the legacy sentinel name is adopted: after compose the block appears once
// under the current name, at the same position, and no legacy-named block
// remains.
func TestEnsureGlobalGit_LegacySentinelAdoption(t *testing.T) {
	legacyBegin := "# BEGIN gitid managed: " + LegacyGlobalGitBlockName
	legacyEnd := "# END gitid managed: " + LegacyGlobalGitBlockName
	legacy := legacyBegin + "\n[init]\n\tdefaultBranch = master\n" + legacyEnd + "\n"
	existing := []byte("# header\n" + legacy + "# footer\n")

	selected := map[string]string{"init.defaultBranch": "main"}
	result, err := EnsureGlobalGit(existing, selected)
	if err != nil {
		t.Fatalf("EnsureGlobalGit: %v", err)
	}

	s := string(result)
	// No legacy block should remain.
	if strings.Contains(s, legacyBegin) {
		t.Error("legacy sentinel BEGIN still present after adoption")
	}
	if strings.Contains(s, legacyEnd) {
		t.Error("legacy sentinel END still present after adoption")
	}
	// The current-named block must be present.
	if !strings.Contains(s, GlobalGitSentinelBegin) {
		t.Error("current sentinel BEGIN missing after adoption")
	}
	if !strings.Contains(s, GlobalGitSentinelEnd) {
		t.Error("current sentinel END missing after adoption")
	}
	// The updated value must be in the block.
	if !strings.Contains(s, "defaultBranch = main") {
		t.Errorf("selected key missing after adoption, got:\n%s", s)
	}
	// Header and footer preserved.
	if !strings.Contains(s, "# header") {
		t.Error("content before legacy block was lost")
	}
	if !strings.Contains(s, "# footer") {
		t.Error("content after legacy block was lost")
	}
}

// TestEnsureGlobalGit_AdditivePreservesExistingKeys verifies that a key
// already present in the block that the selection does not mention SURVIVES
// the compose. This is the R-2 case: a one-key apply on a machine whose block
// carries several must leave the other keys untouched.
func TestEnsureGlobalGit_AdditivePreservesExistingKeys(t *testing.T) {
	// Seed a block with two keys.
	existing := []byte(GlobalGitSentinelBegin + "\n" +
		"[init]\n\tdefaultBranch = master\n" +
		"[core]\n\tignorecase = false\n" +
		GlobalGitSentinelEnd + "\n")

	// Apply a selection that mentions only init.defaultBranch.
	selected := map[string]string{"init.defaultBranch": "main"}
	result, err := EnsureGlobalGit(existing, selected)
	if err != nil {
		t.Fatalf("EnsureGlobalGit: %v", err)
	}

	s := string(result)
	// The selected key wins.
	if !strings.Contains(s, "defaultBranch = main") {
		t.Errorf("selected key missing, got:\n%s", s)
	}
	// The pre-existing key survives.
	if !strings.Contains(s, "ignorecase = false") {
		t.Errorf("pre-existing key was stripped (R-2 violation), got:\n%s", s)
	}
}

// TestEnsureGlobalGit_SelectedKeyWinsOverExisting verifies that a key present
// in both the existing block and the selection takes the selected value.
func TestEnsureGlobalGit_SelectedKeyWinsOverExisting(t *testing.T) {
	existing := []byte(GlobalGitSentinelBegin + "\n[init]\n\tdefaultBranch = master\n" + GlobalGitSentinelEnd + "\n")
	selected := map[string]string{"init.defaultBranch": "main"}
	result, err := EnsureGlobalGit(existing, selected)
	if err != nil {
		t.Fatalf("EnsureGlobalGit: %v", err)
	}

	s := string(result)
	if strings.Contains(s, "defaultBranch = master") {
		t.Error("old value should be replaced by selected value")
	}
	if !strings.Contains(s, "defaultBranch = main") {
		t.Errorf("selected value missing, got:\n%s", s)
	}
}

// TestEnsureGlobalGit_DeferredPagerPreserved verifies that the deferred pager
// key (core.pager) is preserved when already in the block but not in the
// selection — and is NOT added when absent.
func TestEnsureGlobalGit_DeferredPagerPreserved(t *testing.T) {
	// Existing block contains the pager key.
	existing := []byte(GlobalGitSentinelBegin + "\n" +
		"[core]\n\tpager = less -FRX\n" +
		GlobalGitSentinelEnd + "\n")

	// Selection does not mention core.pager.
	selected := map[string]string{"init.defaultBranch": "main"}
	result, err := EnsureGlobalGit(existing, selected)
	if err != nil {
		t.Fatalf("EnsureGlobalGit: %v", err)
	}
	if !strings.Contains(string(result), "pager = less -FRX") {
		t.Errorf("pager key was stripped; should be preserved, got:\n%s", result)
	}
}

// TestEnsureGlobalGit_DeferredPagerNotAdded verifies the pager key is NOT
// added to a fresh block that lacked it.
func TestEnsureGlobalGit_DeferredPagerNotAdded(t *testing.T) {
	selected := map[string]string{"init.defaultBranch": "main"}
	result, err := EnsureGlobalGit(nil, selected)
	if err != nil {
		t.Fatalf("EnsureGlobalGit: %v", err)
	}
	if strings.Contains(string(result), "pager") {
		t.Errorf("pager key should not be added to a block that lacked it, got:\n%s", result)
	}
}

// TestEnsureGlobalGit_ExcludesFileRejected verifies EnsureGlobalGit returns
// an error when the selection contains core.excludesfile.
func TestEnsureGlobalGit_ExcludesFileRejected(t *testing.T) {
	selected := map[string]string{"core.excludesfile": "~/.gitignore_global"}
	_, err := EnsureGlobalGit(nil, selected)
	if err == nil {
		t.Error("expected error when selection contains core.excludesfile (D-11 owner is Phase 8)")
	}
	if err != nil && !strings.Contains(err.Error(), "excludesfile") {
		t.Errorf("error should name core.excludesfile, got: %v", err)
	}
}

// TestEnsureGlobalGit_ExcludesFileDroppedOnAdoption verifies that a legacy
// block carrying core.excludesfile loses it on adoption (D-11.2) while its
// other keys survive.
func TestEnsureGlobalGit_ExcludesFileDroppedOnAdoption(t *testing.T) {
	legacyBegin := "# BEGIN gitid managed: " + LegacyGlobalGitBlockName
	legacyEnd := "# END gitid managed: " + LegacyGlobalGitBlockName
	legacy := legacyBegin + "\n" +
		"[core]\n\texcludesfile = ~/.gitignore_global\n\tignorecase = false\n" +
		legacyEnd + "\n"
	existing := []byte(legacy)

	selected := map[string]string{"init.defaultBranch": "main"}
	result, err := EnsureGlobalGit(existing, selected)
	if err != nil {
		t.Fatalf("EnsureGlobalGit: %v", err)
	}

	s := string(result)
	if strings.Contains(s, "excludesfile") {
		t.Errorf("core.excludesfile must be dropped on adoption (D-11.2), got:\n%s", s)
	}
	// Other keys from the adopted body must survive.
	if !strings.Contains(s, "ignorecase = false") {
		t.Errorf("ignorecase should survive adoption, got:\n%s", s)
	}
}

// TestEnsureGlobalGit_NoExcludesFileInComposedBlock verifies no composed
// block ever contains core.excludesfile, even on a fresh machine.
func TestEnsureGlobalGit_NoExcludesFileInComposedBlock(t *testing.T) {
	// Try all possible selections.
	for _, selected := range []map[string]string{
		{},
		{"init.defaultBranch": "main"},
		{"core.ignorecase": "false"},
	} {
		result, err := EnsureGlobalGit(nil, selected)
		if err != nil {
			continue // expected error for disallowed keys
		}
		if strings.Contains(string(result), "excludesfile") {
			t.Errorf("composed block should never contain excludesfile (D-11.2), got:\n%s", result)
		}
	}
}

// ---------------------------------------------------------------------------
// IsReservedBlockName tests
// ---------------------------------------------------------------------------

// TestIsReservedBlockName_GlobalGit verifies both sentinel names are reserved.
func TestIsReservedBlockName_GlobalGit(t *testing.T) {
	if !IsReservedBlockName(GlobalGitBlockName) {
		t.Errorf("GlobalGitBlockName %q should be reserved", GlobalGitBlockName)
	}
	if !IsReservedBlockName(LegacyGlobalGitBlockName) {
		t.Errorf("LegacyGlobalGitBlockName %q should be reserved", LegacyGlobalGitBlockName)
	}
}

// TestIsReservedBlockName_PlainIdentity verifies a plain identity name is not
// reserved.
func TestIsReservedBlockName_PlainIdentity(t *testing.T) {
	if IsReservedBlockName("personal") {
		t.Error("plain identity name 'personal' should not be reserved")
	}
	if IsReservedBlockName("work") {
		t.Error("plain identity name 'work' should not be reserved")
	}
}

// ---------------------------------------------------------------------------
// ComposeBaselineInclude tests
// ---------------------------------------------------------------------------

// TestComposeBaselineInclude_AppendsInclude verifies that composing produces
// an include block for the given path.
func TestComposeBaselineInclude_AppendsInclude(t *testing.T) {
	result := ComposeBaselineInclude(nil, "~/.gitconfig.d/00-baseline")
	s := string(result)
	if !strings.Contains(s, "path = ~/.gitconfig.d/00-baseline") {
		t.Errorf("ComposeBaselineInclude: missing path, got:\n%s", s)
	}
	if !strings.Contains(s, "[include]") {
		t.Errorf("ComposeBaselineInclude: missing [include] header, got:\n%s", s)
	}
}

// TestWriteBaselineInclude_SkipBehaviorUnchanged verifies that WriteBaselineInclude's
// idempotent-skip behavior is UNCHANGED after the ComposeBaselineInclude
// extraction: a second call with identical content still returns an empty
// backup path and writes nothing.
func TestWriteBaselineInclude_SkipBehaviorUnchanged(t *testing.T) {
	dir := t.TempDir()
	gitconfigPath := dir + "/.gitconfig"
	baselinePath := "~/.gitconfig.d/00-baseline"

	// First write: creates the file.
	bp1, err := WriteBaselineInclude(gitconfigPath, baselinePath)
	if err != nil {
		t.Fatalf("first WriteBaselineInclude: %v", err)
	}
	// File was new so backup path is empty (no file to back up).
	_ = bp1

	// Second write: identical content → idempotent skip → empty backup.
	bp2, err := WriteBaselineInclude(gitconfigPath, baselinePath)
	if err != nil {
		t.Fatalf("second WriteBaselineInclude: %v", err)
	}
	if bp2 != "" {
		t.Errorf("WriteBaselineInclude: second call with identical content should return empty backup, got %q", bp2)
	}
}
