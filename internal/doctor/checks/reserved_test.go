package checks_test

// reserved_test.go is the L4 non-destruction proof for the managed SSH
// artifacts Phase 3 introduces.
//
// D-06 makes the Include'd layout the fresh-machine DEFAULT and D-08 writes the
// macOS `Host *` globals block on EVERY create, so from this phase onward a
// normal machine carries three gitid-owned artifacts the doctor has never seen
// before: the `Include ~/.ssh/config.d/*.config` line, the Include'd
// `config.d/gitid.config` storage file, and the `_global` block. The project's
// recurring L4 failure is shipping such artifacts UNREGISTERED: `health --fix`
// then proposes removing them, the next create re-writes them, and the two
// fight in a destructive false-positive loop.
//
// The test below applies EVERY Fix the Orphans check offers over an Include'd
// fake home and asserts the three artifacts come back byte-identical. The
// non-Include-aware control asserts the opposite, so the guard cannot silently
// rot into a tautology.
//
// Hermetic: t.TempDir() only, never the developer's real $HOME, no network.
// internal/doctor (and this test, which lives under it) must never import
// internal/filewriter (.golangci.yml depguard, D-01), so block removal below is
// an independent, hand-rolled sentinel splice.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/castocolina/gitid/internal/doctor"
	"github.com/castocolina/gitid/internal/doctor/checks"
	"github.com/castocolina/gitid/internal/sshconfig"
)

// includeLine is the gitid-owned Include line D-06 floors at the top of
// ~/.ssh/config. Kept as a literal so the test fails loudly if the canonical
// line ever changes without this guard being revisited.
const includeLine = "Include ~/.ssh/config.d/*.config"

// TestOrphansReservedArtifactsSurviveFix is the L4 proof: over an Include'd
// fake home, CheckOrphans built with Include-AWARE managed-block discovery
// reports nothing about the Phase-3 artifacts, and applying every Fix it does
// offer leaves those artifacts byte-identical.
func TestOrphansReservedArtifactsSurviveFix(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	fx := seedIncludeHome(t, home)

	before := snapshotArtifacts(t, fx)

	names, err := sshconfig.ManagedBlockNames(fx.sshConfigPath)
	if err != nil {
		t.Fatalf("sshconfig.ManagedBlockNames: %v", err)
	}
	deps := orphanDeps(fx, names)

	findings := checks.CheckOrphans(deps)

	// 1. Nothing may be reported ABOUT the reserved artifacts.
	for _, f := range findings {
		for _, reserved := range []string{"ssh-include", "_global", "config.d"} {
			if strings.Contains(f.Title, reserved) || strings.Contains(f.Explanation, reserved) {
				t.Errorf("CheckOrphans reported a finding naming the reserved artifact %q: %s\n%s",
					reserved, f.Title, f.Explanation)
			}
		}
	}

	// 2. The legitimate identity must not be reported either: its Host block
	//    lives in the Include'd file, which Include-aware discovery sees.
	for _, f := range findings {
		if strings.Contains(f.Title, `"personal"`) {
			t.Errorf("CheckOrphans reported the legitimate Include'd identity as an orphan: %s", f.Title)
		}
	}

	// 3. Apply EVERY offered fix. The seeded `ghost` gitconfig block is a real
	//    orphan, so at least one fix must run — an all-nil fix list would make
	//    the byte-identity assertion below vacuous.
	applied := applyEveryFix(t, findings)
	if applied == 0 {
		t.Fatal("no Fix.Fn was offered; the byte-identity assertion below would be vacuous")
	}

	// 4. The three Phase-3 artifacts must be byte-identical afterwards.
	after := snapshotArtifacts(t, fx)
	for name, want := range before {
		if got := after[name]; got != want {
			t.Errorf("reserved artifact %s was mutated by the fix path (L4 violation)\n--- before ---\n%s\n--- after ---\n%s",
				name, want, got)
		}
	}
	// The Include line specifically must still be present and floored.
	if !strings.Contains(after["ssh config"], includeLine) {
		t.Errorf("the gitid Include line was removed by the fix path:\n%s", after["ssh config"])
	}
}

// TestOrphansNonIncludeAwareDepsAreDestructive is the control: the SAME fixture
// with managed-block discovery that reads ~/.ssh/config ALONE reports the
// legitimate identity's gitconfig block as an orphan and offers a fix that
// deletes it. It documents precisely what Include-aware discovery prevents.
// TestOrphansReservedGitRewriteSurvivesFix proves D-11: a provider rewrite is
// managed Git wiring, not an identity, and survives every offered orphan fix.
func TestOrphansReservedGitRewriteSurvivesFix(t *testing.T) {
	home := t.TempDir()
	fx := seedIncludeHome(t, home)
	rewrite := block("provider-rewrite:github.com", "[url \"git@github.com:\"]\n\tinsteadOf = https://github.com/")
	foreign := "[url \"ssh://git@legacy.example/\"]\n\tinsteadOf = https://legacy.example/\n"
	if err := os.WriteFile(fx.gitconfigPath, append(mustReadFile(t, fx.gitconfigPath), []byte(foreign+rewrite)...), 0o600); err != nil {
		t.Fatalf("seeding provider rewrite: %v", err)
	}
	before := string(mustReadFile(t, fx.gitconfigPath))

	names, err := sshconfig.ManagedBlockNames(fx.sshConfigPath)
	if err != nil {
		t.Fatalf("sshconfig.ManagedBlockNames: %v", err)
	}
	deps := orphanDeps(fx, names)
	deps.GitconfigManagedBlockNames = []string{"personal", "ghost", "provider-rewrite:github.com"}
	findings := checks.CheckOrphans(deps)
	for _, finding := range findings {
		if strings.Contains(finding.Title, "provider-rewrite") || strings.Contains(finding.Title, "github.com") {
			t.Errorf("CheckOrphans reported provider rewrite as an identity: %s", finding.Title)
		}
	}
	if applied := applyEveryFix(t, findings); applied == 0 {
		t.Fatal("expected genuine orphan fix to prove reserved assertion is non-vacuous")
	}
	after := string(mustReadFile(t, fx.gitconfigPath))
	if !strings.Contains(after, rewrite) {
		t.Errorf("provider rewrite changed or was deleted by orphan fixes\n--- before ---\n%s\n--- after ---\n%s", before, after)
	}
	if !strings.Contains(after, foreign) {
		t.Errorf("foreign Git content changed by orphan fixes\n--- before ---\n%s\n--- after ---\n%s", before, after)
	}
	if strings.Contains(after, "# BEGIN gitid managed: ghost") {
		t.Errorf("genuine orphan control survived fixes:\n%s", after)
	}
}

// TestOrphansUnreservedGitBlockControl proves the reserved rewrite regression
// does not make CheckOrphans skip genuine managed Git identity blocks.
func TestOrphansUnreservedGitBlockControl(t *testing.T) {
	home := t.TempDir()
	fx := seedIncludeHome(t, home)
	names, err := sshconfig.ManagedBlockNames(fx.sshConfigPath)
	if err != nil {
		t.Fatalf("sshconfig.ManagedBlockNames: %v", err)
	}
	findings := checks.CheckOrphans(orphanDeps(fx, names))
	applyEveryFix(t, findings)
	if got := string(mustReadFile(t, fx.gitconfigPath)); strings.Contains(got, "# BEGIN gitid managed: ghost") {
		t.Errorf("unreserved Git control was not removed:\n%s", got)
	}
}

func TestOrphansNonIncludeAwareDepsAreDestructive(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	fx := seedIncludeHome(t, home)

	// Non-Include-aware discovery: only the main config's own blocks.
	names := blockNames(t, mustReadFile(t, fx.sshConfigPath))
	if containsName(names, "personal") {
		t.Fatal("fixture invalid: the identity block must live in config.d, not in ~/.ssh/config")
	}
	deps := orphanDeps(fx, names)

	findings := checks.CheckOrphans(deps)

	found := false
	for _, f := range findings {
		if strings.Contains(f.Title, `gitconfig block "personal"`) {
			found = true
		}
	}
	if !found {
		t.Fatalf("control case: expected the legitimate identity to be reported as a gitconfig orphan, got %d findings", len(findings))
	}

	applyEveryFix(t, findings)

	gc := mustReadFile(t, fx.gitconfigPath)
	if strings.Contains(string(gc), "# BEGIN gitid managed: personal") {
		t.Error("control case: expected the non-Include-aware fix path to DELETE the legitimate gitconfig block; it survived")
	}
}

// ---------------------------------------------------------------------------
// Fixture
// ---------------------------------------------------------------------------

// includeFixture holds the paths of the seeded fake home.
type includeFixture struct {
	sshDir        string
	sshConfigPath string
	includedPath  string
	gitconfigPath string
}

// seedIncludeHome writes the D-06 Include'd layout under home:
//
//	~/.ssh/config              — the `ssh-include` block + the `_global` block
//	~/.ssh/config.d/gitid.config — the `personal` identity Host block
//	~/.gitconfig               — the `personal` includeIf block + a `ghost` orphan
//
// `ghost` is a genuinely orphaned gitconfig block: it gives the fix path real
// work to do, so "every fix applied" is never an empty loop.
func seedIncludeHome(t *testing.T, home string) includeFixture {
	t.Helper()
	sshDir := filepath.Join(home, ".ssh")
	configDir := filepath.Join(sshDir, "config.d")
	if err := os.MkdirAll(configDir, 0o700); err != nil {
		t.Fatalf("seeding config.d: %v", err)
	}

	fx := includeFixture{
		sshDir:        sshDir,
		sshConfigPath: filepath.Join(sshDir, "config"),
		includedPath:  filepath.Join(configDir, "gitid.config"),
		gitconfigPath: filepath.Join(home, ".gitconfig"),
	}

	sshConfig := block("ssh-include", includeLine) +
		"# a hand-written stanza gitid must never touch\nHost legacy\n  Hostname example.com\n\n" +
		block("_global", "Host *\n  IgnoreUnknown UseKeychain\n  UseKeychain yes\n  AddKeysToAgent yes")
	writeFixture(t, fx.sshConfigPath, sshConfig)

	included := block("personal",
		"Host personal.github.com\n  Hostname ssh.github.com\n  Port 443\n  User git\n"+
			"  IdentityFile "+filepath.Join(sshDir, "id_ed25519_personal")+"\n  IdentitiesOnly yes")
	writeFixture(t, fx.includedPath, included)

	gitconfig := block("personal", "[includeIf \"gitdir:~/git/personal/\"]\n\tpath = ~/.gitconfig.d/personal") +
		block("ghost", "[includeIf \"gitdir:~/git/ghost/\"]\n\tpath = ~/.gitconfig.d/ghost")
	writeFixture(t, fx.gitconfigPath, gitconfig)

	return fx
}

// orphanDeps builds the doctor.Deps CheckOrphans consumes, with a REAL
// RemoveBlock closure that rewrites the fake home's files on disk.
func orphanDeps(fx includeFixture, sshManagedBlockNames []string) doctor.Deps {
	return doctor.Deps{
		SSHConfigPath:              fx.sshConfigPath,
		GitconfigPath:              fx.gitconfigPath,
		SSHManagedBlockNames:       sshManagedBlockNames,
		GitconfigManagedBlockNames: []string{"personal", "ghost"},
		Stat: func(path string) (os.FileInfo, error) {
			return os.Stat(path) //nolint:gosec // hermetic t.TempDir() fixture path (G304)
		},
		RemoveBlock: func(path, name string) error {
			content, err := os.ReadFile(path) //nolint:gosec // hermetic t.TempDir() fixture path (G304)
			if err != nil {
				return err
			}
			return os.WriteFile(path, []byte(removeBlock(string(content), name)), 0o600)
		},
	}
}

// snapshotArtifacts captures the three Phase-3 artifacts' bytes, keyed by a
// human-readable name used in failure output.
func snapshotArtifacts(t *testing.T, fx includeFixture) map[string]string {
	t.Helper()
	return map[string]string{
		"ssh config":            string(mustReadFile(t, fx.sshConfigPath)),
		"config.d/gitid.config": string(mustReadFile(t, fx.includedPath)),
	}
}

// applyEveryFix invokes every Fix.Fn the findings offer and returns how many
// ran. A fix that errors fails the test — a fix path that cannot even execute
// proves nothing about non-destruction.
func applyEveryFix(t *testing.T, findings []doctor.Finding) int {
	t.Helper()
	applied := 0
	for _, f := range findings {
		if f.Fix == nil || f.Fix.Fn == nil {
			continue
		}
		if err := f.Fix.Fn(); err != nil {
			t.Fatalf("applying fix %q: %v", f.Fix.Summary, err)
		}
		applied++
	}
	return applied
}

// ---------------------------------------------------------------------------
// Hand-rolled sentinel helpers (internal/filewriter is depguard-denied here)
// ---------------------------------------------------------------------------

// block wraps body in gitid managed sentinels for name.
func block(name, body string) string {
	return "# BEGIN gitid managed: " + name + "\n" + body + "\n# END gitid managed: " + name + "\n"
}

// blockNames scans content for gitid managed block names, in file order.
func blockNames(t *testing.T, content []byte) []string {
	t.Helper()
	var names []string
	for _, line := range strings.Split(string(content), "\n") {
		if after, ok := strings.CutPrefix(strings.TrimRight(line, "\r"), "# BEGIN gitid managed: "); ok {
			names = append(names, after)
		}
	}
	return names
}

// removeBlock drops the BEGIN..END range for name from content, leaving every
// other line byte-identical.
func removeBlock(content, name string) string {
	begin := "# BEGIN gitid managed: " + name
	end := "# END gitid managed: " + name
	var out []string
	inBlock := false
	for _, line := range strings.SplitAfter(content, "\n") {
		trimmed := strings.TrimRight(line, "\r\n")
		switch {
		case trimmed == begin:
			inBlock = true
		case inBlock && trimmed == end:
			inBlock = false
		case !inBlock:
			out = append(out, line)
		}
	}
	return strings.Join(out, "")
}

// writeFixture writes a fixture file at 0600.
func writeFixture(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("writing fixture %s: %v", path, err)
	}
}

// mustReadFile reads path or fails the test.
func mustReadFile(t *testing.T, path string) []byte {
	t.Helper()
	b, err := os.ReadFile(path) //nolint:gosec // hermetic t.TempDir() fixture path (G304)
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}
	return b
}

// containsName reports whether ss contains want.
func containsName(ss []string, want string) bool {
	for _, s := range ss {
		if s == want {
			return true
		}
	}
	return false
}
