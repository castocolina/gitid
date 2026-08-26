package tuikit

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"reflect"
	"regexp"
	"sort"
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

	if got := len(s.Identities); got != len(stubIdentityRows) {
		t.Fatalf("Seed identities = %d, want %d", got, len(stubIdentityRows))
	}
	if got := len(s.Findings); got != len(stubHealthFindings) {
		t.Fatalf("Seed findings = %d, want %d", got, len(stubHealthFindings))
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
		RotateIdentity{Name: "personal", Backup: "b", ArchivedKeyPath: "p"},
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

func TestReduceRotateIdentity(t *testing.T) {
	s := Seed()
	work := findIdentity(t, s, "work")
	next := Reduce(s, RotateIdentity{
		Name:            "personal",
		Backup:          "~/.ssh/id_ed25519_personal.archive",
		ArchivedKeyPath: "~/.ssh/archive/id_ed25519_personal",
	})
	got := findIdentity(t, next, "personal")
	if got.Note != "Key rotated — previous key archived." {
		t.Errorf("note = %q, want it to name the rotation", got.Note)
	}
	if len(next.Backups) != 1 || next.Backups[0] != "~/.ssh/id_ed25519_personal.archive" {
		t.Errorf("backup not prepended: %v", next.Backups)
	}
	if findIdentity(t, next, "work") != work {
		t.Error("rotate must leave every other identity untouched")
	}
}

func TestReduceRotateIdentityUnknownName(t *testing.T) {
	s := Seed()
	next := Reduce(s, RotateIdentity{Name: "ghost", Backup: "b", ArchivedKeyPath: "p"})
	if !reflect.DeepEqual(next, s) {
		t.Error("rotate of an unknown name must be a no-op")
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

// TestReset pins the SAME contract the pre-extraction TestReduceReset did —
// Reset restores the initial state — at the seam that now owns it. Only the
// Backend knows what "initial" means (fixtures here, a fresh read of the
// user's configuration in the real binary), so Reduce deliberately no longer
// answers Reset and a Backend's Persist MUST (see the Reset doc comment in
// store.go). Both halves of that contract are asserted.
func TestReset(t *testing.T) {
	b := stubBackend{}
	s := b.Persist(Seed(), DeleteIdentity{Name: "personal", Scope: "everything", Backup: "b"})

	if next := b.Persist(s, Reset{}); !reflect.DeepEqual(next, Seed()) {
		t.Error("Reset must restore the seeded state through the Backend")
	}
	if next := Reduce(s, Reset{}); !reflect.DeepEqual(next, s) {
		t.Error("Reduce must leave Reset alone — answering it is the Backend's job")
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

// actionReceiverTypeNames parses src (Go source) and returns every concrete
// type name that declares an isAction() receiver, ignoring pointer markers —
// the source-level truth the AllActions() completeness check compares
// against. go/ast CAN enumerate an interface's implementers by construction;
// reflect cannot (review R-08).
func actionReceiverTypeNames(src string) []string {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "store.go", src, 0)
	if err != nil {
		return nil
	}
	var out []string
	ast.Inspect(f, func(n ast.Node) bool {
		fd, ok := n.(*ast.FuncDecl)
		if !ok || fd.Name.Name != "isAction" || fd.Recv == nil {
			return true
		}
		for _, r := range fd.Recv.List {
			name := exprTypeName(r.Type)
			if name != "" {
				out = append(out, name)
			}
		}
		return true
	})
	return out
}

// exprTypeName reduces a receiver type expression to its base identifier,
// stripping a leading "*" (pointer receiver) so "Foo" and "*Foo" both yield
// "Foo". Returns "" for anything that is not a straight identifier.
func exprTypeName(e ast.Expr) string {
	switch t := e.(type) {
	case *ast.Ident:
		return t.Name
	case *ast.StarExpr:
		return exprTypeName(t.X)
	default:
		return ""
	}
}

// registeredActionTypeNames maps AllActions()'s dynamic type names to a
// membership set.
func registeredActionTypeNames(actions []Action) map[string]bool {
	set := make(map[string]bool)
	for _, a := range actions {
		set[reflect.TypeOf(a).Name()] = true
	}
	return set
}

// TestAllActionsRegistryMatchesDeclaredReceivers is the review-R-08
// completeness gate: the set of isAction() receivers DECLARED in store.go
// must equal the set of dynamic types AllActions() registers — so an action
// that gains an isAction() method without being added to the registry fails
// this test loudly (and a registry entry for a type that no longer exists
// fails too).
func TestAllActionsRegistryMatchesDeclaredReceivers(t *testing.T) {
	src, err := os.ReadFile("store.go")
	if err != nil {
		t.Fatalf("reading store.go: %v", err)
	}
	declared := actionReceiverTypeNames(string(src))

	registered := registeredActionTypeNames(AllActions())

	var extra, missing []string
	sort.Strings(declared)
	for _, name := range declared {
		if !registered[name] {
			missing = append(missing, name)
		}
	}
	declaredSet := make(map[string]bool, len(declared))
	for _, name := range declared {
		declaredSet[name] = true
	}
	for name := range registered {
		if !declaredSet[name] {
			extra = append(extra, name)
		}
	}
	if len(missing) != 0 {
		t.Errorf("isAction() receiver(s) %v declared in store.go but missing from AllActions() — add each to the registry", missing)
	}
	if len(extra) != 0 {
		t.Errorf("AllActions() registers %v with no isAction() receiver in store.go — remove the stale entry", extra)
	}
}

// TestActionRegistryReportsMissingEntry is the negative control for review
// R-08: a fixture source declaring an isAction() receiver that AllActions()
// does NOT register is reported as missing by the same comparison, proving
// the parser-based gate can actually see an omitted type.
func TestActionRegistryReportsMissingEntry(t *testing.T) {
	fixture := `
package fixture
type ExtraAction struct{}
func (ExtraAction) isAction() {}
`
	declared := actionReceiverTypeNames(fixture)
	registered := registeredActionTypeNames(AllActions())

	var missing []string
	for _, name := range declared {
		if !registered[name] {
			missing = append(missing, name)
		}
	}
	if len(declared) == 0 {
		t.Fatal("sanity: the fixture must declare the extra receiver")
	}
	if len(missing) == 0 {
		t.Errorf("the comparison must report an unregistered isAction() receiver; fixture declared %v, registered %v", declared, registered)
	}
}
