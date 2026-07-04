package dummytui

import (
	"reflect"
	"regexp"
	"testing"
)

// findIdentity returns the identity named name from s, failing the test if
// it is absent.
func findIdentity(t *testing.T, s DemoState, name string) DemoIdentity {
	t.Helper()
	for _, row := range s.Identities {
		if row.Name == name {
			return row
		}
	}
	t.Fatalf("identity %q not found in state", name)
	return DemoIdentity{}
}

func hasIdentity(s DemoState, name string) bool {
	for _, row := range s.Identities {
		if row.Name == name {
			return true
		}
	}
	return false
}

func hasFinding(s DemoState, id string) bool {
	for _, f := range s.Findings {
		if f.ID == id {
			return true
		}
	}
	return false
}

func TestSeedMirrorsWebStore(t *testing.T) {
	s := Seed()

	if got := len(s.Identities); got != len(IdentityManagerRows) {
		t.Fatalf("Seed identities = %d, want %d", got, len(IdentityManagerRows))
	}
	if got := len(s.Findings); got != len(HealthFindings) {
		t.Fatalf("Seed findings = %d, want %d", got, len(HealthFindings))
	}

	// Rows WITH a Git fragment get the derived author values (web seed).
	personal := findIdentity(t, s, "personal")
	if personal.GitName != "personal identity" || personal.GitEmail != "you@personal.example" {
		t.Errorf("personal author = %q <%q>, want the web-seed derivation", personal.GitName, personal.GitEmail)
	}
	// Rows WITHOUT a fragment must not fabricate Git values (MGR-03).
	work := findIdentity(t, s, "work")
	if work.GitName != "" || work.GitEmail != "" {
		t.Errorf("work has fabricated Git author values: %q <%q>", work.GitName, work.GitEmail)
	}

	// findingIdentity attribution map (store.ts mirror).
	wantAttribution := map[string]string{
		"ssh-key-perms-archived":           "archived",
		"ssh-identitiesonly-contradiction": "clientB",
		"git-includeif-missing-fragment":   "legacy",
		"git-opensource-no-host-block":     "opensource",
		"ssh-duplicate-host-star":          "", // stays global
	}
	for _, f := range s.Findings {
		if want := wantAttribution[f.ID]; f.Identity != want {
			t.Errorf("finding %s attributed to %q, want %q", f.ID, f.Identity, want)
		}
	}

	if s.Scanned || s.GitBaselineApplied || len(s.SSHApplied) != 0 || len(s.Backups) != 0 {
		t.Error("Seed must start unscanned, baseline unapplied, nothing applied, no backups")
	}
	if s.SSHStorage != StorageSentinel {
		t.Errorf("Seed SSHStorage = %q, want sentinel (STORE-01 default)", s.SSHStorage)
	}
}

// TestReduceNeverMutatesInput asserts on the PRIOR state after every
// action type has been reduced against it.
func TestReduceNeverMutatesInput(t *testing.T) {
	actions := []Action{
		AddIdentity{Identity: DemoIdentity{Name: "acme", State: "complete"}, Backup: "b"},
		ConfigureGit{Name: "work", GitName: "W", GitEmail: "w@x.example", MatchStrategy: "gitdir", Backup: "b"},
		CloneIdentity{Source: "personal", CloneName: "personal-clone"},
		DeleteIdentity{Name: "clientB", Scope: "everything", Backup: "b"},
		DeleteIdentity{Name: "personal", Scope: "git-only", Backup: "b"},
		NewKey{Name: "clientB", Backup: "b"},
		MarkScanned{},
		FixFinding{ID: "git-includeif-missing-fragment", Backup: "b"},
		ApplySSH{Keys: []string{"IdentitiesOnly"}, Backup: "b"},
		ApplyGitBaseline{Backup: "b"},
		EditSSH{Name: "personal", SSHHost: "p.github.com", Hostname: "h", Port: 22, Backup: "b"},
		SetSSHStorage{Layout: StorageInclude, Backup: "b"},
		Reset{},
	}
	for _, action := range actions {
		prior := Seed()
		_ = Reduce(prior, action)
		if !reflect.DeepEqual(prior, Seed()) {
			t.Errorf("Reduce(%T) mutated its input state", action)
		}
	}
}

func TestReduceAddIdentity(t *testing.T) {
	s := Seed()
	next := Reduce(s, AddIdentity{
		Identity: DemoIdentity{Name: "acme", State: "complete", SSHHost: "acme.github.com"},
		Backup:   "~/.ssh/config.backup.X",
	})
	if !hasIdentity(next, "acme") {
		t.Fatal("added identity missing")
	}
	if len(next.Identities) != len(s.Identities)+1 {
		t.Errorf("identity count = %d, want %d", len(next.Identities), len(s.Identities)+1)
	}
	if len(next.Backups) != 1 || next.Backups[0] != "~/.ssh/config.backup.X" {
		t.Errorf("backup not prepended: %v", next.Backups)
	}
}

func TestReduceConfigureGit(t *testing.T) {
	s := Seed()

	// work has an SSH host → completes.
	next := Reduce(s, ConfigureGit{Name: "work", GitName: "Work", GitEmail: "w@work.example", MatchStrategy: "hasconfig", Backup: "b"})
	work := findIdentity(t, next, "work")
	if work.State != "complete" {
		t.Errorf("work state = %q, want complete (has SSH host)", work.State)
	}
	if work.GitFragmentPath != "~/.gitconfig.d/work" || work.MatchStrategy != "hasconfig" {
		t.Errorf("work git side = %q / %q", work.GitFragmentPath, work.MatchStrategy)
	}
	if work.Note != "SSH Host block and Git fragment both present." {
		t.Errorf("work note = %q", work.Note)
	}

	// archived has NO SSH host → git-only.
	next = Reduce(s, ConfigureGit{Name: "archived", GitName: "A", GitEmail: "a@x.example", MatchStrategy: "gitdir", Backup: "b"})
	archived := findIdentity(t, next, "archived")
	if archived.State != "git-only" {
		t.Errorf("archived state = %q, want git-only (no SSH host)", archived.State)
	}
}

func TestReduceCloneIdentity(t *testing.T) {
	s := Seed()
	next := Reduce(s, CloneIdentity{Source: "personal", CloneName: "personal-clone"})
	clone := findIdentity(t, next, "personal-clone")
	if clone.SSHHost != "personal-clone.github.com" {
		t.Errorf("clone SSHHost = %q", clone.SSHHost)
	}
	if clone.KeyPath != "~/.ssh/id_ed25519_personal-clone" {
		t.Errorf("clone KeyPath = %q", clone.KeyPath)
	}
	if clone.GitFragmentPath != "~/.gitconfig.d/personal-clone" {
		t.Errorf("clone fragment = %q", clone.GitFragmentPath)
	}
	if clone.GitName != "personal identity" {
		t.Errorf("clone must copy the Git author (MGR-04); got %q", clone.GitName)
	}
	if clone.Note != `Cloned from "personal" — new key + own Host block, same Git author.` {
		t.Errorf("clone note = %q", clone.Note)
	}

	// Name taken → no-op.
	same := Reduce(s, CloneIdentity{Source: "personal", CloneName: "work"})
	if !reflect.DeepEqual(same, s) {
		t.Error("clone onto a taken name must be a no-op")
	}
	// Missing source → no-op.
	same = Reduce(s, CloneIdentity{Source: "ghost", CloneName: "ghost-clone"})
	if !reflect.DeepEqual(same, s) {
		t.Error("clone of a missing source must be a no-op")
	}
}

func TestReduceDeleteIdentityBothScopes(t *testing.T) {
	s := Seed()

	// everything: drops the row AND its findings.
	next := Reduce(s, DeleteIdentity{Name: "clientB", Scope: "everything", Backup: "b"})
	if hasIdentity(next, "clientB") {
		t.Error("clientB should be gone after delete-everything")
	}
	if hasFinding(next, "ssh-identitiesonly-contradiction") {
		t.Error("clientB's finding should be dropped with the identity")
	}
	if hasFinding(next, "ssh-duplicate-host-star") == false {
		t.Error("global findings must survive an identity delete")
	}

	// git-only: heals to incomplete, keeps the row.
	next = Reduce(s, DeleteIdentity{Name: "personal", Scope: "git-only", Backup: "b"})
	personal := findIdentity(t, next, "personal")
	if personal.State != "incomplete" {
		t.Errorf("personal state = %q, want incomplete", personal.State)
	}
	if personal.GitFragmentPath != "" || personal.GitName != "" || personal.GitEmail != "" || personal.MatchStrategy != "" {
		t.Error("git-only delete must clear the Git side")
	}
	if personal.Note != "SSH Host block present; Git identity was deleted." {
		t.Errorf("note = %q", personal.Note)
	}
	if personal.SSHHost == "" {
		t.Error("git-only delete must keep the SSH side")
	}
}

func TestReduceNewKey(t *testing.T) {
	s := Seed()
	next := Reduce(s, NewKey{Name: "clientB", Backup: "b"})
	clientB := findIdentity(t, next, "clientB")
	if clientB.KeyPath != "~/.ssh/id_ed25519_clientB" {
		t.Errorf("KeyPath = %q", clientB.KeyPath)
	}
	if clientB.State != "incomplete" {
		t.Errorf("clientB (key-missing, no fragment) should heal to incomplete; got %q", clientB.State)
	}
	if clientB.Note != "New key generated; Host block re-points at it." {
		t.Errorf("note = %q", clientB.Note)
	}
}

func TestReduceMarkScanned(t *testing.T) {
	next := Reduce(Seed(), MarkScanned{})
	if !next.Scanned {
		t.Error("MarkScanned must set Scanned")
	}
}

func TestReduceFixFinding(t *testing.T) {
	s := Seed()

	// Legacy healing: the includeIf fix flips "legacy" to complete.
	next := Reduce(s, FixFinding{ID: "git-includeif-missing-fragment", Backup: "b"})
	if hasFinding(next, "git-includeif-missing-fragment") {
		t.Error("fixed finding must disappear")
	}
	legacy := findIdentity(t, next, "legacy")
	if legacy.State != "complete" {
		t.Errorf("legacy state = %q, want complete (healed)", legacy.State)
	}
	if legacy.KeyPath != "~/.ssh/id_ed25519_legacy" {
		t.Errorf("legacy KeyPath = %q, want the default fill-in", legacy.KeyPath)
	}
	if legacy.Note != "Fragment restored — SSH Host block and Git fragment both present." {
		t.Errorf("legacy note = %q", legacy.Note)
	}

	// Plain fix: only the finding disappears.
	next = Reduce(s, FixFinding{ID: "ssh-duplicate-host-star", Backup: "b"})
	if hasFinding(next, "ssh-duplicate-host-star") {
		t.Error("fixed finding must disappear")
	}
	if len(next.Identities) != len(s.Identities) {
		t.Error("plain fix must not change identities")
	}

	// Unknown id: no-op.
	same := Reduce(s, FixFinding{ID: "ghost", Backup: "b"})
	if !reflect.DeepEqual(same, s) {
		t.Error("fixing an unknown finding must be a no-op")
	}
}

func TestReduceApplySSHDedupes(t *testing.T) {
	s := Seed()
	next := Reduce(s, ApplySSH{Keys: []string{"IdentitiesOnly", "HashKnownHosts"}, Backup: "b"})
	next = Reduce(next, ApplySSH{Keys: []string{"IdentitiesOnly", "StrictHostKeyChecking"}, Backup: "b"})
	want := []string{"IdentitiesOnly", "HashKnownHosts", "StrictHostKeyChecking"}
	if !reflect.DeepEqual(next.SSHApplied, want) {
		t.Errorf("SSHApplied = %v, want %v (set union)", next.SSHApplied, want)
	}
}

func TestReduceApplyGitBaseline(t *testing.T) {
	next := Reduce(Seed(), ApplyGitBaseline{Backup: "b"})
	if !next.GitBaselineApplied {
		t.Error("ApplyGitBaseline must set the flag")
	}
}

func TestReduceEditSSH(t *testing.T) {
	next := Reduce(Seed(), EditSSH{Name: "personal", SSHHost: "p2.github.com", Hostname: "alt.github.com", Port: 22, Backup: "b"})
	personal := findIdentity(t, next, "personal")
	if personal.SSHHost != "p2.github.com" || personal.Hostname != "alt.github.com" || personal.Port != 22 {
		t.Errorf("edit-ssh did not apply: %+v", personal)
	}
}

func TestReduceSetSSHStorageRoundTrips(t *testing.T) {
	s := Seed()
	next := Reduce(s, SetSSHStorage{Layout: StorageInclude, Backup: "b"})
	if next.SSHStorage != StorageInclude {
		t.Errorf("layout = %q, want include", next.SSHStorage)
	}
	back := Reduce(next, SetSSHStorage{Layout: StorageSentinel, Backup: "b"})
	if back.SSHStorage != StorageSentinel {
		t.Errorf("layout = %q, want sentinel (reversible, STORE-03)", back.SSHStorage)
	}
}

func TestReduceReset(t *testing.T) {
	s := Reduce(Seed(), DeleteIdentity{Name: "personal", Scope: "everything", Backup: "b"})
	next := Reduce(s, Reset{})
	if !reflect.DeepEqual(next, Seed()) {
		t.Error("Reset must restore the seeded state")
	}
}

func TestFindingCountsAndRollupPinSeededChip(t *testing.T) {
	s := Seed()
	counts := CountFindings(s)
	if counts.Warnings != 1 || counts.Errors != 3 {
		t.Errorf("seeded counts = !%d ✗%d, want !1 ✗3 (chip `8 ids · ! 1 ✗ 3`)", counts.Warnings, counts.Errors)
	}
	if HealthRollup(s) != "error" {
		t.Errorf("rollup = %q, want error", HealthRollup(s))
	}

	// All-clean variant.
	clean := s
	clean.Findings = nil
	counts = CountFindings(clean)
	if counts.Warnings != 0 || counts.Errors != 0 {
		t.Errorf("clean counts = %+v", counts)
	}
	if HealthRollup(clean) != "healthy" {
		t.Errorf("clean rollup = %q, want healthy", HealthRollup(clean))
	}
}

func TestFindingsFor(t *testing.T) {
	s := Seed()
	legacy := FindingsFor(s, "legacy")
	if len(legacy) != 1 || legacy[0].ID != "git-includeif-missing-fragment" {
		t.Errorf("FindingsFor(legacy) = %v", legacy)
	}
	if got := FindingsFor(s, "work"); len(got) != 0 {
		t.Errorf("FindingsFor(work) = %v, want none", got)
	}
}

func TestNewBackupPathShape(t *testing.T) {
	got := NewBackupPath("~/.ssh/config")
	want := regexp.MustCompile(`^~/\.ssh/config\.backup\.\d{4}-\d{2}-\d{2}T\d{2}-\d{2}-\d{2}Z$`)
	if !want.MatchString(got) {
		t.Errorf("NewBackupPath = %q, want timestamped `<file>.backup.<stamp>` shape", got)
	}
}
