package checks_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/castocolina/gitid/internal/doctor"
	"github.com/castocolina/gitid/internal/doctor/checks"
	"github.com/castocolina/gitid/internal/identity"
	"github.com/castocolina/gitid/internal/sshconfig"
)

// orphFileInfo is a minimal os.FileInfo for orphans tests.
type orphFileInfo struct{ mode os.FileMode }

func (o orphFileInfo) Name() string       { return "" }
func (o orphFileInfo) Size() int64        { return 0 }
func (o orphFileInfo) Mode() os.FileMode  { return o.mode }
func (o orphFileInfo) ModTime() time.Time { return time.Time{} }
func (o orphFileInfo) IsDir() bool        { return false }
func (o orphFileInfo) Sys() interface{}   { return nil }

// orphStat returns a Stat function where only the given paths are present (0600).
func orphStat(presentPaths ...string) func(string) (os.FileInfo, error) {
	set := make(map[string]bool, len(presentPaths))
	for _, p := range presentPaths {
		set[p] = true
	}
	return func(path string) (os.FileInfo, error) {
		if set[path] {
			return orphFileInfo{mode: 0o600}, nil
		}
		return nil, os.ErrNotExist
	}
}

// TestOrphanFragment: a fragment file exists on disk but no gitconfig includeIf managed
// block claims its identity name → Orphans warning with [fix].
func TestOrphanFragment(t *testing.T) {
	fragPath := "/home/u/.gitconfig.d/stale"
	d := doctor.Deps{
		// stale fragment is on disk
		Stat:       orphStat(fragPath),
		Identities: []identity.Account{}, // no accounts → the block is genuinely orphaned
		// gitconfig managed blocks DO include "stale" — the block exists in gitconfig
		// but has no corresponding SSH side (the algorithm checks fragment-on-disk but
		// no owning includeIf block claims it → orphan).
		// Wait: the orphan is: fragment file on disk, no gitconfig block claims it.
		// If GitconfigManagedBlockNames does NOT contain "stale", but we know "stale"
		// key exists → orphan. We simulate: SSHManagedBlockNames has "stale" (SSH block
		// present) but GitconfigManagedBlockNames does NOT have "stale".
		SSHManagedBlockNames:       []string{"stale"},
		GitconfigManagedBlockNames: []string{}, // no matching includeIf
		// AllSSHHostIdentityFiles has no reference to the stale key.
		AllSSHHostIdentityFiles: []string{},
		KeyPaths:                []string{},
		// RemoveBlock and SSHConfigPath wired so Fix descriptor is non-nil (D-11).
		SSHConfigPath: "/home/u/.ssh/config",
		RemoveBlock:   func(_, _ string) error { return nil },
	}

	findings := checks.CheckOrphans(d)

	if len(findings) == 0 {
		t.Fatal("expected at least one Orphans finding for orphaned SSH Host block, got none")
	}
	var found bool
	for _, f := range findings {
		if f.Family != doctor.FamilyOrphans {
			t.Errorf("finding family = %q, want %q", f.Family, doctor.FamilyOrphans)
		}
		if f.Severity == doctor.SeverityWarning {
			found = true
			if f.Fix == nil {
				t.Error("orphaned block finding must carry a Fix descriptor ([fix] D-11)")
			}
		}
	}
	if !found {
		t.Errorf("expected warning-severity Orphans finding, got: %v", orphTitles(findings))
	}
}

// TestOrphanAliasHostNoInclude: a managed SSH Host block name exists but no matching
// gitconfig includeIf block claims it → Orphans warning with [fix].
func TestOrphanAliasHostNoInclude(t *testing.T) {
	d := doctor.Deps{
		Stat:       orphStat(), // nothing on disk
		Identities: []identity.Account{},
		// SSH managed block "old" exists, but gitconfig has no corresponding block.
		SSHManagedBlockNames:       []string{"old"},
		GitconfigManagedBlockNames: []string{},
		AllSSHHostIdentityFiles:    []string{},
		KeyPaths:                   []string{},
		// RemoveBlock and SSHConfigPath wired so Fix descriptor is non-nil (D-11).
		SSHConfigPath: "/home/u/.ssh/config",
		RemoveBlock:   func(_, _ string) error { return nil },
	}

	findings := checks.CheckOrphans(d)

	if len(findings) == 0 {
		t.Fatal("expected Orphans finding for SSH block with no includeIf, got none")
	}
	var found bool
	for _, f := range findings {
		if f.Severity == doctor.SeverityWarning && orphContains(f.Title, "old") {
			found = true
			if f.Fix == nil {
				t.Error("orphaned SSH block finding must carry a Fix descriptor (D-11)")
			}
		}
	}
	if !found {
		t.Errorf("expected orphaned block finding mentioning 'old', got: %v", orphTitles(findings))
	}
}

// TestOrphanReservedBaselineNotFlagged: the reserved baseline-include gitconfig
// block has no SSH Host block by design and MUST NOT be reported as an orphan.
// Flagging it produces a removal [fix] that deletes the legitimate baseline
// include, fighting the Baseline check's restore in an endless loop.
func TestOrphanReservedBaselineNotFlagged(t *testing.T) {
	d := doctor.Deps{
		Stat:       orphStat(),
		Identities: []identity.Account{},
		// Only the reserved baseline-include block is present in gitconfig; no SSH side.
		SSHManagedBlockNames:       []string{},
		GitconfigManagedBlockNames: []string{"baseline-include"},
		AllSSHHostIdentityFiles:    []string{},
		KeyPaths:                   []string{},
		GitconfigPath:              "/home/u/.gitconfig",
		RemoveBlock:                func(_, _ string) error { return nil },
	}

	findings := checks.CheckOrphans(d)

	for _, f := range findings {
		if orphContains(f.Title, "baseline-include") {
			t.Errorf("reserved baseline-include must not be reported as an orphan, got: %q", f.Title)
		}
	}
	if len(findings) != 0 {
		t.Errorf("expected no orphan findings for a lone reserved block, got: %v", orphTitles(findings))
	}
}

// TestOrphanReservedSSHIncludeNotFlagged: the reserved ssh-include SSH managed
// block has no gitconfig includeIf counterpart by design and MUST NOT be
// reported as an orphan (Pitfall 4 / project memory "Doctor reserved-block
// false-positive loop" — the SSH-side mirror of
// TestOrphanReservedBaselineNotFlagged). Flagging it produces a removal [fix]
// that deletes the legitimate Include line, fighting EnsureIncludeLine's
// restore in an endless loop.
func TestOrphanReservedSSHIncludeNotFlagged(t *testing.T) {
	d := doctor.Deps{
		Stat:       orphStat(),
		Identities: []identity.Account{},
		// Only the reserved ssh-include block is present in ~/.ssh/config; no
		// gitconfig side.
		SSHManagedBlockNames:       []string{"ssh-include"},
		GitconfigManagedBlockNames: []string{},
		AllSSHHostIdentityFiles:    []string{},
		KeyPaths:                   []string{},
		SSHConfigPath:              "/home/u/.ssh/config",
		RemoveBlock:                func(_, _ string) error { return nil },
	}

	findings := checks.CheckOrphans(d)

	for _, f := range findings {
		if orphContains(f.Title, "ssh-include") {
			t.Errorf("reserved ssh-include must not be reported as an orphan, got: %q", f.Title)
		}
	}
	if len(findings) != 0 {
		t.Errorf("expected no orphan findings for a lone reserved SSH block, got: %v", orphTitles(findings))
	}
}

// TestOrphanReservedGlobalsNotFlagged: neither globals block name is a
// deletable identity artifact. A lone "global-ssh" (or legacy "_global") SSH
// managed block has no gitconfig counterpart by design — gitid's wildcard
// stanza is SSH-only wiring — and MUST NOT be reported as an orphan under
// either registered sentinel name (D-08, Task 1). Flagging it produces a
// removal [fix] that deletes the stanza, fighting EnsureGlobals' restore in an
// endless loop — the same reserved-block false-positive loop this file
// documents for ssh-include.
func TestOrphanReservedGlobalsNotFlagged(t *testing.T) {
	for _, name := range []string{sshconfig.GlobalBlockName, sshconfig.LegacyGlobalBlockName} {
		d := doctor.Deps{
			Stat:       orphStat(),
			Identities: []identity.Account{},
			// Only the reserved globals block is present in ~/.ssh/config; no
			// gitconfig side.
			SSHManagedBlockNames:       []string{name},
			GitconfigManagedBlockNames: []string{},
			AllSSHHostIdentityFiles:    []string{},
			KeyPaths:                   []string{},
			SSHConfigPath:              "/home/u/.ssh/config",
			RemoveBlock:                func(_, _ string) error { return nil },
		}

		findings := checks.CheckOrphans(d)

		for _, f := range findings {
			if orphContains(f.Title, name) {
				t.Errorf("reserved globals block %q must not be reported as an orphan, got: %q", name, f.Title)
			}
		}
		if len(findings) != 0 {
			t.Errorf("expected no orphan findings for a lone reserved %q block, got: %v", name, orphTitles(findings))
		}
	}
}

// TestOrphanKey: a gitid key file exists on disk but is referenced by NO Host block
// (managed or hand-written) → warning, no [fix], honest wording.
func TestOrphanKey(t *testing.T) {
	keyPath := "/home/u/.ssh/gitid_stale"
	d := doctor.Deps{
		// Key file exists on disk.
		Stat:                       orphStat(keyPath),
		Identities:                 []identity.Account{},
		SSHManagedBlockNames:       []string{},
		GitconfigManagedBlockNames: []string{},
		// AllSSHHostIdentityFiles does NOT include this key — no Host block references it.
		AllSSHHostIdentityFiles: []string{},
		KeyPaths:                []string{keyPath},
	}

	findings := checks.CheckOrphans(d)

	if len(findings) == 0 {
		t.Fatal("expected Orphans finding for unreferenced key, got none")
	}
	var found bool
	for _, f := range findings {
		if f.Family == doctor.FamilyOrphans && f.Severity == doctor.SeverityWarning &&
			orphContains(f.Title, "not referenced") {
			found = true
			if f.Fix != nil {
				t.Error("unused-key finding must NOT carry a Fix descriptor (D-03/D-13 report-only)")
			}
			// D-13: honest wording admitting gitid cannot confirm it is unused.
			if !orphContains(f.Explanation, "review") && !orphContains(f.Explanation, "direct server") {
				t.Errorf("unused-key explanation must mention 'review' or 'direct server SSH'; got: %q", f.Explanation)
			}
		}
	}
	if !found {
		t.Errorf("expected 'not referenced' Orphans finding for key, got: %v", orphTitles(findings))
	}
}

// TestOrphanKeyReferencedByHandWrittenHost: a key IS referenced by a hand-written
// (non-managed) Host block → must NOT be flagged (D-12 respects user config).
func TestOrphanKeyReferencedByHandWrittenHost(t *testing.T) {
	keyPath := "/home/u/.ssh/gitid_personal"
	d := doctor.Deps{
		Stat:                       orphStat(keyPath),
		Identities:                 []identity.Account{},
		SSHManagedBlockNames:       []string{},
		GitconfigManagedBlockNames: []string{},
		// Hand-written Host block references this key.
		AllSSHHostIdentityFiles: []string{keyPath},
		KeyPaths:                []string{keyPath},
	}

	findings := checks.CheckOrphans(d)

	for _, f := range findings {
		if f.Family == doctor.FamilyOrphans && orphContains(f.Title, keyPath) {
			t.Errorf("key referenced by hand-written Host must NOT be flagged; got: %q", f.Title)
		}
	}
}

// TestOrphanNotIncomplete: an account with Incomplete set produces NO Orphans
// findings — that belongs to Coherence (Pitfall 5).
func TestOrphanNotIncomplete(t *testing.T) {
	keyPath := "/home/u/.ssh/gitid_broken"
	acct := makeAccount("broken", "broken.github.com", "broken@example.com",
		keyPath, "", "fragment-file")
	d := doctor.Deps{
		Stat:                       orphStat(keyPath),
		Identities:                 []identity.Account{acct},
		SSHManagedBlockNames:       []string{"broken"},
		GitconfigManagedBlockNames: []string{"broken"},
		AllSSHHostIdentityFiles:    []string{keyPath},
		KeyPaths:                   []string{keyPath},
	}

	findings := checks.CheckOrphans(d)

	for _, f := range findings {
		if f.Family == doctor.FamilyOrphans {
			t.Errorf("Incomplete account must NOT produce Orphans findings; got: %q", f.Title)
		}
	}
}

// TestOrphanAllPass: every artifact owned by a block, every key referenced → no findings.
func TestOrphanAllPass(t *testing.T) {
	keyPath := "/home/u/.ssh/gitid_work"
	acct := makeAccount("work", "work.github.com", "work@example.com",
		keyPath, "/home/u/.gitconfig.d/work", "")
	d := doctor.Deps{
		Stat:       orphStat(keyPath),
		Identities: []identity.Account{acct},
		// SSH block "work" is paired with gitconfig block "work".
		SSHManagedBlockNames:       []string{"work"},
		GitconfigManagedBlockNames: []string{"work"},
		// Key IS referenced by a Host block.
		AllSSHHostIdentityFiles: []string{keyPath},
		KeyPaths:                []string{keyPath},
	}

	findings := checks.CheckOrphans(d)

	if len(findings) != 0 {
		t.Errorf("expected zero Orphans findings for all-paired artifacts, got %d: %v",
			len(findings), orphTitles(findings))
	}
}

// TestOrphansArchivedKeyExcludedByBuildInventory is the D-06 doctor-reserved
// regression test (review R-04): an archived key pair, planted inside the
// dedicated gitid-archive directory, must never surface as a doctor orphan
// finding.
//
// The test is CAUSAL, not incidental: it builds Inventory via
// identity.BuildInventory with an INJECTED ListKeyFiles seam that returns
// the archived path DIRECTLY, bypassing the production "id_*" glob (which
// is non-recursive and would never even match a path under gitid-archive/
// in the first place — a test that only relied on the glob could never go
// red, per 05-RESEARCH.md). CheckOrphans then consumes Inventory.UnusedKeys
// as its KeyPaths, so the exclusion is proven at the exact seam a real
// doctor composition root would use.
func TestOrphansArchivedKeyExcludedByBuildInventory(t *testing.T) {
	home := t.TempDir()
	sshDir := filepath.Join(home, ".ssh")
	archiveDir := sshconfig.ArchiveDir(sshDir)
	if err := os.MkdirAll(archiveDir, 0o700); err != nil {
		t.Fatalf("seeding archive dir: %v", err)
	}
	archivedKey := filepath.Join(archiveDir, "id_ed25519_work.170000000000000001")
	if err := os.WriteFile(archivedKey, []byte("archived-private-key\n"), 0o600); err != nil { //nolint:gosec // hermetic t.TempDir() fixture
		t.Fatalf("seeding archived key: %v", err)
	}

	deps := identity.InventoryDepsForHome(home)
	deps.ListKeyFiles = func() ([]string, error) { return []string{archivedKey}, nil }

	inv, err := identity.BuildInventory(deps)
	if err != nil {
		t.Fatalf("identity.BuildInventory: %v", err)
	}
	if len(inv.UnusedKeys) != 0 {
		t.Fatalf("expected zero UnusedKeys for an archived key, got %v", inv.UnusedKeys)
	}

	orphDeps := doctor.Deps{
		Stat:     orphStat(archivedKey),
		KeyPaths: inv.UnusedKeys,
	}
	findings := checks.CheckOrphans(orphDeps)
	for _, f := range findings {
		if orphContains(f.Title, archivedKey) {
			t.Errorf("archived key must not produce an orphan finding, got: %q", f.Title)
		}
	}
}

// TestOrphansArchivedKeyNegativeControl proves the guard above is load-
// bearing: with IsReservedKeyPath disabled, the SAME planted archive key
// DOES surface as an orphan finding — the negative control review R-04
// requires so this test suite can never be vacuously green.
func TestOrphansArchivedKeyNegativeControl(t *testing.T) {
	home := t.TempDir()
	sshDir := filepath.Join(home, ".ssh")
	archiveDir := sshconfig.ArchiveDir(sshDir)
	if err := os.MkdirAll(archiveDir, 0o700); err != nil {
		t.Fatalf("seeding archive dir: %v", err)
	}
	archivedKey := filepath.Join(archiveDir, "id_ed25519_work.170000000000000001")
	if err := os.WriteFile(archivedKey, []byte("archived-private-key\n"), 0o600); err != nil { //nolint:gosec // hermetic t.TempDir() fixture
		t.Fatalf("seeding archived key: %v", err)
	}

	deps := identity.InventoryDepsForHome(home)
	deps.ListKeyFiles = func() ([]string, error) { return []string{archivedKey}, nil }
	deps.IsReservedKeyPath = func(string) bool { return false } // the negative control

	inv, err := identity.BuildInventory(deps)
	if err != nil {
		t.Fatalf("identity.BuildInventory: %v", err)
	}
	if len(inv.UnusedKeys) != 1 || inv.UnusedKeys[0] != archivedKey {
		t.Fatalf("expected the archived key in UnusedKeys with the guard disabled, got %v", inv.UnusedKeys)
	}

	orphDeps := doctor.Deps{
		Stat:     orphStat(archivedKey),
		KeyPaths: inv.UnusedKeys,
	}
	findings := checks.CheckOrphans(orphDeps)
	var found bool
	for _, f := range findings {
		if orphContains(f.Title, archivedKey) {
			found = true
		}
	}
	if !found {
		t.Errorf("expected the archived key to produce an orphan finding with the guard disabled, got: %v", orphTitles(findings))
	}
}

// orphContains reports whether s contains sub.
func orphContains(s, sub string) bool {
	if len(sub) == 0 {
		return true
	}
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

// orphTitles extracts titles for test error messages.
func orphTitles(findings []doctor.Finding) []string {
	out := make([]string, len(findings))
	for i, f := range findings {
		out[i] = f.Title
	}
	return out
}
