package main

// identity_test.go covers the D-01 command tree (Task 3) and the D-03 read
// surface (Task 2): the shared identityVerb/newVerbCmd constructor, the
// reserved noun groups, and identity_read.go's record projection + output
// writers.

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/castocolina/gitid/internal/identity"
	"github.com/castocolina/gitid/internal/tester"
	"github.com/castocolina/gitid/internal/tuikit"
)

// ---------------------------------------------------------------------------
// Task 3 — the D-01 command tree.
// ---------------------------------------------------------------------------

// TestRootCommandTreeTopLevelSurface walks newRootCmd() and asserts the
// expected top-level Use names are each registered EXACTLY once — a future
// phase adding a verb cannot silently shadow an alias.
func TestRootCommandTreeTopLevelSurface(t *testing.T) {
	root := newRootCmd()
	counts := map[string]int{}
	for _, c := range root.Commands() {
		counts[c.Name()]++
	}
	for _, want := range []string{"identity", "ssh", "git", "health", "fix", "debug", "list", "show", "delete"} {
		if counts[want] != 1 {
			t.Errorf("expected top-level command %q exactly once, got %d", want, counts[want])
		}
	}
}

// TestNoDuplicateFullyQualifiedCommandPaths walks the WHOLE tree and asserts
// no two commands share the same fully-qualified path.
func TestNoDuplicateFullyQualifiedCommandPaths(t *testing.T) {
	root := newRootCmd()
	root.InitDefaultCompletionCmd()
	root.InitDefaultHelpCmd()

	seen := map[string]bool{}
	var walk func(cmd *cobra.Command)
	walk = func(cmd *cobra.Command) {
		path := cmd.CommandPath()
		if seen[path] {
			t.Errorf("duplicate command path: %s", path)
		}
		seen[path] = true
		for _, c := range cmd.Commands() {
			walk(c)
		}
	}
	walk(root)
}

// TestReservedNounGroupsReturnPhaseNamedErrors asserts each reserved noun
// group's RunE returns a non-nil error naming the implementing phase (D-01).
func TestReservedNounGroupsReturnPhaseNamedErrors(t *testing.T) {
	root := newRootCmd()
	cases := map[string]string{
		"git":    "Phase 7",
		"health": "Phase 8",
		"fix":    "Phase 8",
	}
	for use, wantPhase := range cases {
		cmd, _, err := root.Find([]string{use})
		if err != nil {
			t.Fatalf("Find(%q): %v", use, err)
		}
		if cmd.RunE == nil {
			t.Fatalf("%q has no RunE", use)
		}
		rerr := cmd.RunE(cmd, nil)
		if rerr == nil {
			t.Errorf("%q RunE returned nil, want a not-yet-implemented error", use)
			continue
		}
		if !strings.Contains(rerr.Error(), wantPhase) {
			t.Errorf("%q RunE error = %q, want it to name %q", use, rerr.Error(), wantPhase)
		}
	}
}

// TestVerbSpecsProduceIdenticalNounAndFlatCommands proves review R-15: for
// EVERY identity verb spec, the noun-form command and the flat-alias command
// — two DISTINCT *cobra.Command objects built from the SAME spec — expose
// flag sets with identical names/shorthands/defaults, and identical Args
// behavior for a zero-argument and a two-argument invocation.
func TestVerbSpecsProduceIdenticalNounAndFlatCommands(t *testing.T) {
	for _, spec := range identityVerbSpecs() {
		spec := spec
		t.Run(spec.use, func(t *testing.T) {
			nounCmd := newVerbCmd(spec)
			flatCmd := newVerbCmd(spec)

			if got, want := flagSignature(nounCmd), flagSignature(flatCmd); !equalStringSlices(got, want) {
				t.Errorf("flag signatures differ:\nnoun: %v\nflat: %v", got, want)
			}

			for _, args := range [][]string{nil, {"a", "b"}} {
				nounErr := nounCmd.Args(nounCmd, args)
				flatErr := flatCmd.Args(flatCmd, args)
				if (nounErr == nil) != (flatErr == nil) {
					t.Errorf("Args(%v) disagreement: noun err=%v, flat err=%v", args, nounErr, flatErr)
				}
			}
		})
	}
}

// flagSignature returns a sorted "name|shorthand|default" list for every
// flag on cmd's own flag set.
func flagSignature(cmd *cobra.Command) []string {
	var out []string
	cmd.Flags().VisitAll(func(f *pflag.Flag) {
		out = append(out, f.Name+"|"+f.Shorthand+"|"+f.DefValue)
	})
	sort.Strings(out)
	return out
}

func equalStringSlices(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// TestVerbNounAndFlatCommandsDoNotDelegateToEachOther injects a recording
// run closure into a spec and invokes each constructed command independently
// — proving neither command's RunE calls the other's (review R-15): each
// invocation records EXACTLY one call.
func TestVerbNounAndFlatCommandsDoNotDelegateToEachOther(t *testing.T) {
	calls := 0
	spec := identityVerb{
		use:  "probe",
		args: cobra.NoArgs,
		run: func(*cobra.Command, []string) error {
			calls++
			return nil
		},
	}
	nounCmd := newVerbCmd(spec)
	flatCmd := newVerbCmd(spec)

	nounCmd.SetArgs(nil)
	if err := nounCmd.Execute(); err != nil {
		t.Fatalf("nounCmd.Execute: %v", err)
	}
	if calls != 1 {
		t.Fatalf("calls after noun invocation = %d, want 1", calls)
	}

	flatCmd.SetArgs(nil)
	if err := flatCmd.Execute(); err != nil {
		t.Fatalf("flatCmd.Execute: %v", err)
	}
	if calls != 2 {
		t.Fatalf("calls after flat invocation = %d, want 2 (each command's own RunE ran exactly once)", calls)
	}
}

// ---------------------------------------------------------------------------
// Task 2 — the D-03 read surface.
// ---------------------------------------------------------------------------

// TestIdentityRecordEightLabelsAcrossAxes builds one fixture per MGR-02
// label combination and asserts each of the EIGHT labels appears in at least
// one record's identity_state or key_state, while the collapsed `state`
// field observes exactly SEVEN distinct values with key-used-both absent
// (ClassifyState's documented precedence — see collapseState).
func TestIdentityRecordEightLabelsAcrossAxes(t *testing.T) {
	fixtures := []identity.IdentityHealth{
		{Name: "a", IdentityState: identity.StateComplete, KeyState: identity.StateKeyMissing},
		{Name: "b", IdentityState: identity.StateComplete, KeyState: identity.StateKeyUnused},
		{Name: "c", IdentityState: identity.StateComplete, KeyState: identity.StateKeyUsedSSHOnly},
		{Name: "d", IdentityState: identity.StateComplete, KeyState: identity.StateKeyUsedBoth},
		{Name: "e", IdentityState: identity.StateIncomplete, KeyState: identity.StateKeyUsedBoth},
		{Name: "f", IdentityState: identity.StateGitOnly, KeyState: identity.StateKeyUsedBoth},
		{Name: "g", IdentityState: identity.StateFragmentPathMissing, KeyState: identity.StateKeyUsedBoth},
	}

	seenAxis := map[string]bool{}
	seenCollapsed := map[string]bool{}
	for _, h := range fixtures {
		rec := toIdentityRecord(h, identity.Account{Name: h.Name})
		seenAxis[rec.IdentityState] = true
		seenAxis[rec.KeyState] = true
		seenCollapsed[rec.State] = true
	}

	wantLabels := []string{
		string(identity.StateComplete), string(identity.StateIncomplete), string(identity.StateGitOnly),
		string(identity.StateFragmentPathMissing), string(identity.StateKeyUnused),
		string(identity.StateKeyUsedSSHOnly), string(identity.StateKeyUsedBoth), string(identity.StateKeyMissing),
	}
	for _, want := range wantLabels {
		if !seenAxis[want] {
			t.Errorf("label %q never appears in identity_state or key_state across the fixture set", want)
		}
	}

	if _, present := seenCollapsed[string(identity.StateKeyUsedBoth)]; present {
		t.Error("collapsed state must never observe key-used-both — ClassifyState's collapse always folds it into complete")
	}
	if len(seenCollapsed) != 7 {
		t.Errorf("collapsed state observed %d distinct values, want exactly 7: %v", len(seenCollapsed), seenCollapsed)
	}
}

// TestIdentityRecordSSHOnlyEmptyGitFields asserts an SSH-only identity
// (no fragment, no gitconfig includeIf block) marshals git_name, git_email,
// match_strategy, and fragment_path as empty strings — never a fabricated
// default (MGR-03).
func TestIdentityRecordSSHOnlyEmptyGitFields(t *testing.T) {
	h := identity.IdentityHealth{Name: "sshonly", IdentityState: identity.StateIncomplete, KeyState: identity.StateKeyUsedSSHOnly}
	acct := identity.Account{Name: "sshonly", Alias: "sshonly.github.com"} // no FragmentPath, no Matches, no GitName/GitEmail
	rec := toIdentityRecord(h, acct)

	if rec.GitName != "" || rec.GitEmail != "" || rec.MatchStrategy != "" || rec.FragmentPath != "" {
		t.Errorf("SSH-only record carries a non-empty Git field: %+v", rec)
	}

	out, err := json.Marshal(rec)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	for _, field := range []string{`"git_name":""`, `"git_email":""`, `"match_strategy":""`, `"fragment_path":""`} {
		if !strings.Contains(string(out), field) {
			t.Errorf("marshaled record missing %s: %s", field, out)
		}
	}
}

// TestWriteTableHasHeaderTabDelimitedDoesNot drives writeTable,
// writeTabDelimited, and writeJSON against the same fixture record set.
func TestWriteTableHasHeaderTabDelimitedDoesNot(t *testing.T) {
	records := []identityRecord{
		{Name: "work", State: "complete", Alias: "work.github.com", GitEmail: "work@example.com", KeyState: string(identity.StateKeyUsedSSHOnly)},
	}

	var table bytes.Buffer
	writeTable(&table, records)
	if !strings.Contains(table.String(), "NAME") {
		t.Errorf("writeTable output has no header row:\n%s", table.String())
	}

	var tabbed bytes.Buffer
	writeTabDelimited(&tabbed, records)
	if strings.Contains(tabbed.String(), "NAME") {
		t.Errorf("writeTabDelimited output must not carry a header row:\n%s", tabbed.String())
	}
	if !strings.Contains(tabbed.String(), "work\tcomplete") {
		t.Errorf("writeTabDelimited output missing the expected tab-separated record:\n%s", tabbed.String())
	}

	var jsonBuf bytes.Buffer
	if err := writeJSON(&jsonBuf, identityListDocument{Identities: records, UnusedKeys: []string{}}); err != nil {
		t.Fatalf("writeJSON: %v", err)
	}
	var doc identityListDocument
	if err := json.Unmarshal(jsonBuf.Bytes(), &doc); err != nil {
		t.Fatalf("json.Unmarshal: %v (raw: %s)", err, jsonBuf.String())
	}
	if len(doc.Identities) != 1 || doc.Identities[0].Name != "work" {
		t.Errorf("writeJSON round-trip mismatch: %+v", doc)
	}
}

// TestUnusedKeysMarshalsAsEmptyArrayNeverNull asserts unused_keys marshals
// as [] (never null) when the inventory reports none.
func TestUnusedKeysMarshalsAsEmptyArrayNeverNull(t *testing.T) {
	var buf bytes.Buffer
	if err := writeJSON(&buf, identityListDocument{Identities: nil, UnusedKeys: []string{}}); err != nil {
		t.Fatalf("writeJSON: %v", err)
	}
	if !strings.Contains(buf.String(), `"unused_keys": []`) {
		t.Errorf(`expected "unused_keys": [], got: %s`, buf.String())
	}
	if strings.Contains(buf.String(), `"unused_keys": null`) {
		t.Errorf("unused_keys marshaled as null: %s", buf.String())
	}
}

// TestBuildIdentityRecords_UnusedKeyNotInAnyRecord seeds an on-disk key
// referenced by no Host block and asserts it appears in unused_keys and in
// NO identity record.
func TestBuildIdentityRecords_UnusedKeyNotInAnyRecord(t *testing.T) {
	home := t.TempDir()
	seedDeleteFixture(t, home, "work")
	// An orphan private key: no Host block anywhere references it.
	writeFile(t, filepath.Join(home, ".ssh", "id_ed25519_orphan"), "stub-priv\n")

	records, unused, err := buildIdentityRecords(home)
	if err != nil {
		t.Fatalf("buildIdentityRecords: %v", err)
	}

	orphanPath := filepath.Join(home, ".ssh", "id_ed25519_orphan")
	found := false
	for _, k := range unused {
		if k == orphanPath {
			found = true
		}
	}
	if !found {
		t.Errorf("unused_keys = %v, want it to contain %q", unused, orphanPath)
	}
	for _, r := range records {
		if r.Name == "orphan" {
			t.Error("the orphan key must never appear as its own identity record")
		}
	}
}

// TestBuildIdentityRecords_UnionNeverDoubleCountsAClassifiedIdentity is the
// WR-13 regression's non-regression half: after switching
// buildIdentityRecords to build the record set from the UNION of
// inv.Identities and b.accounts() (keyed by name), a normally-classified
// identity (present in BOTH sets, the common case) must still appear
// EXACTLY ONCE, fully classified — never duplicated by the union logic,
// and never demoted to unclassifiedIdentityRecord's empty-state fallback.
func TestBuildIdentityRecords_UnionNeverDoubleCountsAClassifiedIdentity(t *testing.T) {
	home := t.TempDir()
	seedDeleteFixture(t, home, "work")

	records, _, err := buildIdentityRecords(home)
	if err != nil {
		t.Fatalf("buildIdentityRecords: %v", err)
	}
	var matches int
	for _, r := range records {
		if r.Name != "work" {
			continue
		}
		matches++
		if r.State == "" {
			t.Errorf("WR-13: a normally-classified identity must not fall through to the unclassified fallback: %+v", r)
		}
	}
	if matches != 1 {
		t.Errorf("WR-13: identity %q appears %d times in records, want exactly 1", "work", matches)
	}
}

// TestUnclassifiedIdentityRecordNeverFabricatesState is the WR-13
// regression's direct unit proof: unclassifiedIdentityRecord — the fallback
// buildIdentityRecords now uses for an account b.accounts() reconstructed
// but identity.BuildInventory's classification omitted — must never
// fabricate a specific State/IdentityState/KeyState or report Complete,
// since no classification actually ran. It must still carry every
// Account-shaped fact so `identity show` can describe what it knows.
func TestUnclassifiedIdentityRecordNeverFabricatesState(t *testing.T) {
	acct := identity.Account{
		Name: "ghost", Alias: "ghost.github.com", Hostname: "ssh.github.com", Port: 443,
		Provider: "github.com", KeyPath: "~/.ssh/id_ed25519_ghost", PubPath: "~/.ssh/id_ed25519_ghost.pub",
		FragmentPath: "~/.gitconfig.d/ghost", GitName: "Ghost User", GitEmail: "ghost@example.com",
	}
	r := unclassifiedIdentityRecord(acct)
	if r.Name != "ghost" {
		t.Errorf("Name = %q, want %q", r.Name, "ghost")
	}
	if r.State != "" || r.IdentityState != "" || r.KeyState != "" {
		t.Errorf("WR-13: unclassifiedIdentityRecord must not fabricate a state — got State=%q IdentityState=%q KeyState=%q", r.State, r.IdentityState, r.KeyState)
	}
	if r.Complete {
		t.Error("WR-13: an unclassified identity must never report Complete = true")
	}
	if r.Problems == nil {
		t.Error("Problems must be a non-nil empty slice, never null (MGR-03/D-03 JSON contract)")
	}
	if r.Alias != acct.Alias || r.Hostname != acct.Hostname || r.GitEmail != acct.GitEmail || r.KeyPath != acct.KeyPath {
		t.Errorf("unclassifiedIdentityRecord must still carry every Account-shaped fact: %+v", r)
	}
}

// TestBuildIdentityRecords_NoWriteBetweenCalls asserts two consecutive
// buildIdentityRecords calls with no intervening write produce byte-identical
// records (MGR-08 — nothing is cached or persisted between calls).
func TestBuildIdentityRecords_NoWriteBetweenCalls(t *testing.T) {
	home := t.TempDir()
	seedDeleteFixture(t, home, "work")

	first, firstUnused, err := buildIdentityRecords(home)
	if err != nil {
		t.Fatalf("first buildIdentityRecords: %v", err)
	}
	second, secondUnused, err := buildIdentityRecords(home)
	if err != nil {
		t.Fatalf("second buildIdentityRecords: %v", err)
	}

	firstJSON, _ := json.Marshal(identityListDocument{Identities: first, UnusedKeys: firstUnused})
	secondJSON, _ := json.Marshal(identityListDocument{Identities: second, UnusedKeys: secondUnused})
	if string(firstJSON) != string(secondJSON) {
		t.Errorf("two consecutive reads diverged:\nfirst:  %s\nsecond: %s", firstJSON, secondJSON)
	}
}

// TestIdentityShowUnknownName asserts `show` on an unknown name returns a
// non-nil error naming the requested identity, and writes nothing to the
// output writer.
func TestIdentityShowUnknownName(t *testing.T) {
	home := t.TempDir()
	seedDeleteFixture(t, home, "work")
	t.Setenv("HOME", home)

	cmd := newVerbCmd(newIdentityShowVerb())
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"ghost"})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("show ghost must return a non-nil error")
	}
	if !strings.Contains(err.Error(), "ghost") {
		t.Errorf("error = %v, want it to name the requested identity", err)
	}
	if out.Len() != 0 {
		t.Errorf("show on an unknown name wrote %q, want nothing", out.String())
	}
}

// TestIdentityListJSONRoundTrip is a light integration proof (the e2e suite
// covers the compiled binary): running the list verb with --json produces
// output that unmarshals into identityListDocument with the seeded identity
// present.
func TestIdentityListJSONRoundTrip(t *testing.T) {
	home := t.TempDir()
	seedDeleteFixture(t, home, "work")
	t.Setenv("HOME", home)

	cmd := newVerbCmd(newIdentityListVerb())
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"--json"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("list --json: %v", err)
	}

	var doc identityListDocument
	if err := json.Unmarshal(out.Bytes(), &doc); err != nil {
		t.Fatalf("json.Unmarshal: %v (raw: %s)", err, out.String())
	}
	if len(doc.Identities) != 1 || doc.Identities[0].Name != "work" {
		t.Errorf("identities = %+v, want exactly one record named work", doc.Identities)
	}
	if doc.UnusedKeys == nil {
		t.Error("unused_keys must never be nil in the decoded document")
	}
}

// ---------------------------------------------------------------------------
// Plan 05-08 Task 1 — the D-02 adaptive-depth resolver and the write verbs.
// ---------------------------------------------------------------------------

// cliTestCmd builds a bare cobra command with captured stdout/stderr/stdin for
// driving the CLI handler functions headlessly.
func cliTestCmd() (*cobra.Command, *bytes.Buffer, *bytes.Buffer) {
	cmd := &cobra.Command{Use: "gitid-test"}
	var out, errb bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errb)
	cmd.SetIn(strings.NewReader(""))
	return cmd, &out, &errb
}

// ioDiscard is a minimal io.Writer used to silence cobra output in read tests.
type ioDiscard struct{}

func (ioDiscard) Write(p []byte) (int, error) { return len(p), nil }

// testRepoRoot walks up from the package working directory to the repository
// root (the go.mod location), mirroring the e2e harness's own repoRoot so
// source-level and docs-parity tests can resolve repository files.
func testRepoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd: %v", err)
	}
	for {
		if _, statErr := os.Stat(filepath.Join(dir, "go.mod")); statErr == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatalf("go.mod not found walking up from %s", dir)
		}
		dir = parent
	}
}

// snapshotSeams captures every test-only lifecycle/gate seam's current value
// (create, gate, rotate, repair, delete, connectivity-test) and returns the
// restore closure.
func snapshotSeams() func() {
	oldCreate, oldGate := commitCreateInto, cliPreWriteGate
	oldRotate, oldRepair := cliRotateInto, cliRepairInto
	oldDelete, oldTest := cliDeleteInto, cliConnectivityTest
	oldApply, oldMigrate, oldSSHTUI := cliGlobalSSHApplyInto, cliSSHStorageMigrateInto, sshTUILaunch
	return func() {
		commitCreateInto, cliPreWriteGate = oldCreate, oldGate
		cliRotateInto, cliRepairInto = oldRotate, oldRepair
		cliDeleteInto, cliConnectivityTest = oldDelete, oldTest
		cliGlobalSSHApplyInto, cliSSHStorageMigrateInto, sshTUILaunch = oldApply, oldMigrate, oldSSHTUI
	}
}

// seamGuard registers restoration of every seam for the duration of the test.
// Call it BEFORE installing any override.
func seamGuard(t *testing.T) {
	t.Helper()
	t.Cleanup(snapshotSeams())
}

// passStageMsg builds a canned accepted WizardStageMsg a recording
// cliPreWriteGate double returns.
func passStageMsg(outcome tuikit.TestOutcome) tuikit.WizardStageMsg {
	return tuikit.WizardStageMsg{Stage: 1, Result: tuikit.TestResultView{Outcome: outcome}}
}

// managedFixturePaths returns every managed file plus both key files a
// seedDeleteFixture home carries — the byte-identity set for the regression
// tests.
func managedFixturePaths(home, name string) []string {
	return []string{
		filepath.Join(home, ".ssh", "config"),
		filepath.Join(home, ".ssh", "allowed_signers"),
		filepath.Join(home, ".ssh", "id_ed25519_"+name),
		filepath.Join(home, ".ssh", "id_ed25519_"+name+".pub"),
		filepath.Join(home, ".gitconfig"),
		filepath.Join(home, ".gitconfig.d", name),
	}
}

// --- depthResolver ---------------------------------------------------------

// TestDepthResolverAcrossAllTerminalCombinations drives the resolver across
// complete and incomplete flag sets crossed with ALL FOUR combinations of
// stdinTTY and stdoutTTY, and asserts the pre-filled-TUI outcome occurs ONLY
// for true/true (review R-21).
func TestDepthResolverAcrossAllTerminalCombinations(t *testing.T) {
	required := []string{"name", "provider"}
	cases := []struct {
		name      string
		supplied  map[string]bool
		stdinTTY  bool
		stdoutTTY bool
		want      resolveOutcome
		wantMiss  string
	}{
		{"complete / no tty", map[string]bool{"name": true, "provider": true}, false, false, resolveHeadless, ""},
		{"complete / stdin-only tty", map[string]bool{"name": true, "provider": true}, true, false, resolveHeadless, ""},
		{"complete / stdout-only tty", map[string]bool{"name": true, "provider": true}, false, true, resolveHeadless, ""},
		{"complete / both tty", map[string]bool{"name": true, "provider": true}, true, true, resolveHeadless, ""},
		{"missing / no tty", map[string]bool{"name": true}, false, false, resolveMissingFlags, "provider"},
		{"missing / stdin-only tty", map[string]bool{"name": true}, true, false, resolveMissingFlags, "provider"},
		{"missing / stdout-only tty", map[string]bool{"name": true}, false, true, resolveMissingFlags, "provider"},
		{"missing / both tty", map[string]bool{"name": true}, true, true, resolvePrefilledTUI, "provider"},
		{"missing two / both tty", map[string]bool{}, true, true, resolvePrefilledTUI, "name, provider"},
		{"missing two / no tty", map[string]bool{}, false, false, resolveMissingFlags, "name, provider"},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			out, missing := depthResolver{required: required, supplied: tc.supplied, stdinTTY: tc.stdinTTY, stdoutTTY: tc.stdoutTTY}.resolve()
			if out != tc.want {
				t.Errorf("resolve() outcome = %v, want %v (stdinTTY=%v stdoutTTY=%v)", out, tc.want, tc.stdinTTY, tc.stdoutTTY)
			}
			if tc.wantMiss == "" {
				if len(missing) != 0 {
					t.Errorf("resolve() missing = %v, want none", missing)
				}
			} else if strings.Join(missing, ", ") != tc.wantMiss {
				t.Errorf("resolve() missing = %v, want %q", missing, tc.wantMiss)
			}
		})
	}
}

// TestIdentityCreateMissingFlagsErrorNamesOnlyMissing asserts the
// non-terminal incomplete case returns an error whose message contains every
// missing flag name and no flag name that was supplied.
func TestIdentityCreateMissingFlagsErrorNamesOnlyMissing(t *testing.T) {
	cmd, _, _ := cliTestCmd()
	supplied := identityCreateFlags{Name: "work", Provider: "github.com", GitName: "Work Worker"} // git-email missing
	err := runIdentityCreate(cmd, supplied, false, false)
	if err == nil {
		t.Fatal("an incomplete create without a terminal must exit non-zero")
	}
	msg := err.Error()
	if !strings.Contains(msg, "git-email") {
		t.Errorf("error = %q, want it to name the missing flag git-email", msg)
	}
	for _, absent := range []string{"provider", "git-name", "name"} {
		if strings.Contains(msg, absent) {
			t.Errorf("error = %q, must not name supplied flag %q", msg, absent)
		}
	}
}

// TestIdentityCreateRejectsPathTraversalName is the CR-04 regression:
// createInputFromCreateFlags validated only the SSH host block
// (sshconfig.ValidateHostBlock), never the identity NAME itself.
// validateToken (inside ValidateHostBlock) rejects whitespace and shell
// metacharacters but not '/' or '..', so an unvalidated name flowed straight
// into FragmentPath = filepath.Join(b.fragmentDir, name) — "--name
// '../.bashrc'" resolved the fragment write to ~/.bashrc, still "inside"
// $HOME by containedRegularPath's own (weaker) check.
func TestIdentityCreateRejectsPathTraversalName(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	cmd, _, _ := cliTestCmd()
	flags := identityCreateFlags{
		Name:     "../.bashrc",
		Provider: "github.com",
		GitName:  "Attacker",
		GitEmail: "attacker@example.com",
		Yes:      true,
	}
	err := runIdentityCreate(cmd, flags, false, false)
	if err == nil {
		t.Fatal("identity create with a path-traversal name must be refused, not written")
	}
	if !strings.Contains(err.Error(), "invalid identity name") {
		t.Errorf("error = %v, want it to name the identity-name validation failure", err)
	}
	if _, statErr := os.Stat(filepath.Join(home, ".bashrc")); !os.IsNotExist(statErr) {
		t.Errorf("identity create must not write outside the managed fragment directory: statErr=%v", statErr)
	}
}

// TestIdentityCreateRejectsInvalidGitEmail is CR-04's second gap:
// createInputFromCreateFlags never called identity.ValidateEmail, so a
// malformed --git-email reached WriteFragment unchecked and was only caught
// later, deep inside gitconfig.validateEmail — by then the SSH block and
// key had already been written and the whole transaction had to roll back
// (proven pre-fix: the error names "gitconfig: user.email is malformed" and
// lists ~12 restored paths, including the generated key pair). The fix must
// refuse at the flag boundary, before ~/.ssh even exists.
func TestIdentityCreateRejectsInvalidGitEmail(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	cmd, _, _ := cliTestCmd()
	flags := identityCreateFlags{
		Name:     "work",
		Provider: "github.com",
		GitName:  "Work User",
		GitEmail: "not-an-email",
		Yes:      true,
	}
	err := runIdentityCreate(cmd, flags, false, false)
	if err == nil {
		t.Fatal("identity create with an invalid git-email must be refused before any write")
	}
	if !strings.Contains(err.Error(), "invalid email") {
		t.Errorf("error = %v, want the early ValidateEmail rejection (\"invalid email\"), not a deep-write rollback", err)
	}
	if _, statErr := os.Stat(filepath.Join(home, ".ssh")); !os.IsNotExist(statErr) {
		t.Errorf("identity create must not create ~/.ssh before email validation: statErr=%v", statErr)
	}
}

// --- delete: exactly one scope flag ----------------------------------------

// TestIdentityDeleteRequiresExactlyOneScopeFlag asserts supplying BOTH scope
// flags or NEITHER is an error in every mode.
func TestIdentityDeleteRequiresExactlyOneScopeFlag(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	seedDeleteFixture(t, home, "work")

	cmd, _, _ := cliTestCmd()
	if err := runIdentityDelete(cmd, "work", identityDeleteFlags{GitOnly: true, All: true, Yes: true}, false, false); err == nil {
		t.Error("passing both --git-only and --all must be an error")
	}
	if err := runIdentityDelete(cmd, "work", identityDeleteFlags{Yes: true}, false, false); err == nil {
		t.Error("passing neither scope flag must be an error in non-interactive mode")
	}
	if err := runIdentityDelete(cmd, "work", identityDeleteFlags{}, true, true); err == nil {
		t.Error("passing neither scope flag must be an error in interactive mode too")
	}
}

// TestConfirmDeleteRequiresTypedNameForEverythingScope is the WR-01
// regression: for the identical irreversible everything-scope delete, the
// TUI requires FixDestructive{ConfirmWord: plan.Name} (typing the identity
// name), while the CLI's interactive prompt accepted the generic "yes" used
// by every other verb — the stronger gate was dropped exactly where the
// blast radius is largest. git-only delete must still accept "yes".
func TestConfirmDeleteRequiresTypedNameForEverythingScope(t *testing.T) {
	cmd, _, _ := cliTestCmd()
	cmd.SetIn(strings.NewReader("yes\n"))
	ok, err := confirmDelete(cmd, "work", identity.DeleteScopeEverything)
	if err != nil {
		t.Fatalf("confirmDelete: %v", err)
	}
	if ok {
		t.Error("typing the generic \"yes\" must NOT confirm an everything-scope delete — the identity name is required")
	}

	cmd2, _, _ := cliTestCmd()
	cmd2.SetIn(strings.NewReader("work\n"))
	ok2, err := confirmDelete(cmd2, "work", identity.DeleteScopeEverything)
	if err != nil {
		t.Fatalf("confirmDelete: %v", err)
	}
	if !ok2 {
		t.Error("typing the identity name must confirm an everything-scope delete")
	}

	cmd3, _, _ := cliTestCmd()
	cmd3.SetIn(strings.NewReader("yes\n"))
	ok3, err := confirmDelete(cmd3, "work", identity.DeleteScopeGitOnly)
	if err != nil {
		t.Fatalf("confirmDelete: %v", err)
	}
	if !ok3 {
		t.Error("git-only scope must still accept the generic \"yes\"")
	}
}

// TestIdentityDeleteRefusesWhenPlanFails is the CR-05 regression: PlanDelete
// is deliberately fail-closed — a plan that failed to read a scan source
// must never be indistinguishable from a legitimately small plan — and the
// TUI honors that (refreshDeletePlan blanks the plan, disables the confirm
// control). The CLI used to do the opposite: `if plan, perr :=
// b.DeletePlan(...); perr == nil { render }` silently fell through to the
// irreversible delete on any plan error, printing nothing (defeating the
// T-05-38 disclosure the code comment above it claims to satisfy). The fix
// must refuse the delete outright when the plan cannot be built.
func TestIdentityDeleteRefusesWhenPlanFails(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	seedDeleteFixture(t, home, "work")

	allowed := filepath.Join(home, ".ssh", "allowed_signers")
	if err := os.Chmod(allowed, 0o000); err != nil {
		t.Fatalf("chmod 0000 allowed_signers: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(allowed, 0o600) })

	cmd, _, _ := cliTestCmd()
	err := runIdentityDelete(cmd, "work", identityDeleteFlags{All: true, Yes: true}, false, false)
	if err == nil {
		t.Fatal("identity delete must refuse when the delete plan cannot be built, not proceed silently")
	}
	if !strings.Contains(err.Error(), "delete plan") {
		t.Errorf("error = %v, want it to name the delete-plan failure", err)
	}
	if _, statErr := os.Stat(filepath.Join(home, ".ssh", "config")); os.IsNotExist(statErr) {
		t.Error("identity delete must not have deleted anything when the plan could not be built")
	}
	if _, statErr := os.Stat(filepath.Join(home, ".ssh", "id_ed25519_work")); os.IsNotExist(statErr) {
		t.Error("identity delete must not have removed the key pair when the plan could not be built")
	}
}

// --- confirmation flag still backs up ----------------------------------------

// TestIdentityDeleteWithYesStillBacksUp runs a write verb with the
// confirmation flag over a fake home with pre-existing targets and asserts a
// timestamped backup file exists (D-02: --yes suppresses ONLY the prompt,
// never the backup).
func TestIdentityDeleteWithYesStillBacksUp(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	seedDeleteFixture(t, home, "work")

	cmd, out, _ := cliTestCmd()
	if err := runIdentityDelete(cmd, "work", identityDeleteFlags{GitOnly: true, Yes: true}, false, false); err != nil {
		t.Fatalf("delete --git-only --yes: %v", err)
	}
	if !strings.Contains(out.String(), "backed up ->") {
		t.Errorf("expected a backup receipt; output:\n%s", out.String())
	}
	var backupCount int
	err := filepath.Walk(home, func(path string, info os.FileInfo, werr error) error {
		if werr != nil {
			return werr
		}
		if !info.IsDir() && strings.Contains(filepath.Base(path), ".bak.") {
			backupCount++
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walking home for backups: %v", err)
	}
	if backupCount == 0 {
		t.Errorf("no timestamped backup file exists after a --yes run (D-02)")
	}
}

// --- the unauthorized-delete regression (review R2-03) -----------------------

// TestIdentityDeleteUnauthorizedNoYesLeavesBytesIdentical is review R2-03's
// regression: `identity delete <scope> <name>` with stdinTTY=false,
// stdoutTTY=false and no --yes exits non-zero, its message names the
// confirmation flag, NO lifecycle function is invoked (recording double count
// zero), and every seeded managed file plus both key files are byte-identical
// afterwards.
func TestIdentityDeleteUnauthorizedNoYesLeavesBytesIdentical(t *testing.T) {
	for _, kind := range []struct {
		name  string
		flags identityDeleteFlags
	}{
		{"--all", identityDeleteFlags{All: true}},
		{"--git-only", identityDeleteFlags{GitOnly: true}},
	} {
		kind := kind
		t.Run(kind.name, func(t *testing.T) {
			seamGuard(t)
			home := t.TempDir()
			t.Setenv("HOME", home)
			seedDeleteFixture(t, home, "work")
			managed := managedFixturePaths(home, "work")
			before := snapshotPaths(t, managed)

			var lifecycleCalls int
			cliDeleteInto = func(_ *realBackend, _ string, _ identity.DeleteScope, _ lifecyclePolicy) (lifecycleResult, error) {
				lifecycleCalls++
				return lifecycleResult{}, nil
			}

			cmd, _, _ := cliTestCmd()
			err := runIdentityDelete(cmd, "work", kind.flags, false, false)
			if err == nil {
				t.Fatal("unauthorized non-interactive delete must exit non-zero")
			}
			if !strings.Contains(err.Error(), "--yes") {
				t.Errorf("error = %q, want it to name the confirmation flag --yes", err.Error())
			}
			if lifecycleCalls != 0 {
				t.Errorf("lifecycle invoked %d times on an unauthorized delete, want 0", lifecycleCalls)
			}
			assertUnchanged(t, before, snapshotPaths(t, managed))
		})
	}
}

// TestIdentityKeyVerbUnauthorizedNoYesLeavesBytesIdentical is the rotate and
// new-key half of review R2-03's sibling cases: non-interactive, no --yes,
// zero lifecycle invocations, byte-identical files.
func TestIdentityKeyVerbUnauthorizedNoYesLeavesBytesIdentical(t *testing.T) {
	for _, verb := range []string{"rotate", "new-key"} {
		verb := verb
		t.Run(verb, func(t *testing.T) {
			seamGuard(t)
			home := t.TempDir()
			t.Setenv("HOME", home)
			seedDeleteFixture(t, home, "work")
			managed := managedFixturePaths(home, "work")
			before := snapshotPaths(t, managed)

			var lifecycleCalls int
			cliRotateInto = func(_ *realBackend, _ string, _ lifecyclePolicy) (lifecycleResult, error) {
				lifecycleCalls++
				return lifecycleResult{}, nil
			}
			cliRepairInto = func(_ *realBackend, _ string, _ lifecyclePolicy) (lifecycleResult, error) {
				lifecycleCalls++
				return lifecycleResult{}, nil
			}

			cmd, _, _ := cliTestCmd()
			err := runIdentityKeyVerb(cmd, "work", verb, identityKeyFlags{}, false, false)
			if err == nil {
				t.Fatalf("unauthorized non-interactive %s must exit non-zero", verb)
			}
			if !strings.Contains(err.Error(), "--yes") {
				t.Errorf("error = %q, want it to name the confirmation flag --yes", err.Error())
			}
			if lifecycleCalls != 0 {
				t.Errorf("lifecycle invoked %d times on an unauthorized %s, want 0", lifecycleCalls, verb)
			}
			assertUnchanged(t, before, snapshotPaths(t, managed))
		})
	}
}

// TestIdentityDeleteWithYesCompletesAndDryRunWritesNothing: the same
// invocation WITH --yes completes and produces a backup (and an archive for
// the everything scope); the same invocation with --dry-run and no --yes
// exits ZERO having written nothing.
func TestIdentityDeleteWithYesCompletesAndDryRunWritesNothing(t *testing.T) {
	t.Run("with --yes completes and reports a backup", func(t *testing.T) {
		seamGuard(t)
		home := t.TempDir()
		t.Setenv("HOME", home)
		seedDeleteFixture(t, home, "work")

		cmd, out, _ := cliTestCmd()
		if err := runIdentityDelete(cmd, "work", identityDeleteFlags{All: true, Yes: true}, false, false); err != nil {
			t.Fatalf("delete --all --yes over a seeded home: %v", err)
		}
		if !strings.Contains(out.String(), "backed up ->") {
			t.Errorf("expected a backup receipt after --yes; output:\n%s", out.String())
		}
		if _, statErr := os.Stat(filepath.Join(home, ".ssh", "gitid-archive")); statErr != nil {
			t.Errorf("an everything-scope delete should archive the key pair; statErr=%v", statErr)
		}
		if _, statErr := os.Stat(filepath.Join(home, ".ssh", "id_ed25519_work")); !os.IsNotExist(statErr) {
			t.Errorf("everything-scope delete must remove the live private key; statErr=%v", statErr)
		}
	})

	t.Run("--dry-run without --yes exits zero having written nothing", func(t *testing.T) {
		seamGuard(t)
		home := t.TempDir()
		t.Setenv("HOME", home)
		seedDeleteFixture(t, home, "work")
		managed := managedFixturePaths(home, "work")
		before := snapshotPaths(t, managed)

		var lifecycleCalls int
		cliDeleteInto = func(_ *realBackend, _ string, _ identity.DeleteScope, _ lifecyclePolicy) (lifecycleResult, error) {
			lifecycleCalls++
			return lifecycleResult{}, nil
		}

		cmd, _, _ := cliTestCmd()
		if err := runIdentityDelete(cmd, "work", identityDeleteFlags{All: true, DryRun: true}, false, false); err != nil {
			t.Fatalf("delete --all --dry-run without --yes must exit ZERO: %v", err)
		}
		if lifecycleCalls != 0 {
			t.Errorf("a dry run must never reach the lifecycle; invoked %d times", lifecycleCalls)
		}
		assertUnchanged(t, before, snapshotPaths(t, managed))
	})
}

// TestIdentityKeyVerbDryRunNoYesWritesNothing is the rotate/new-key dry-run
// sibling: --dry-run without --yes exits ZERO and creates no file anywhere
// under the fake HOME (including the staging directory).
func TestIdentityKeyVerbDryRunNoYesWritesNothing(t *testing.T) {
	for _, verb := range []string{"rotate", "new-key"} {
		verb := verb
		t.Run(verb, func(t *testing.T) {
			seamGuard(t)
			home := t.TempDir()
			t.Setenv("HOME", home)
			seedDeleteFixture(t, home, "work")
			before := homeFileListing(t, home)

			cliConnectivityTest = func(_ *realBackend, _ string) tester.Result {
				return tester.Result{Outcome: tester.PASS}
			}

			cmd, out, _ := cliTestCmd()
			if err := runIdentityKeyVerb(cmd, "work", verb, identityKeyFlags{DryRun: true}, false, false); err != nil {
				t.Fatalf("dry run without --yes must exit zero: %v", err)
			}
			if !strings.Contains(out.String(), "dry run: would "+verb) {
				t.Errorf("the ceremony plan was not printed; output:\n%s", out.String())
			}
			after := homeFileListing(t, home)
			if len(before) != len(after) {
				t.Fatalf("%s dry run created files under HOME: before=%v after=%v", verb, before, after)
			}
			for i := range before {
				if before[i] != after[i] {
					t.Fatalf("%s dry run changed the HOME file listing: %q -> %q", verb, before, after)
				}
			}
		})
	}
}

// --- rotate dry-run labels the CURRENT key (review R2-13) -------------------

// TestIdentityRotateDryRunNamesCurrentKeyAndCaveat pins the R2-13 contract:
// the dry-run output names the identity's CURRENT key as what was tested,
// contains the frozen post-rotation caveat sentence, and makes no claim about
// the replacement key's reachability.
func TestIdentityRotateDryRunNamesCurrentKeyAndCaveat(t *testing.T) {
	seamGuard(t)
	home := t.TempDir()
	t.Setenv("HOME", home)
	seedDeleteFixture(t, home, "work")

	cliConnectivityTest = func(_ *realBackend, _ string) tester.Result {
		return tester.Result{Outcome: tester.PASS}
	}

	cmd, out, _ := cliTestCmd()
	if err := runIdentityKeyVerb(cmd, "work", "rotate", identityKeyFlags{DryRun: true}, false, false); err != nil {
		t.Fatalf("rotate --dry-run: %v", err)
	}
	got := out.String()
	if !strings.Contains(got, "id_ed25519_work") {
		t.Errorf("output must name the identity's CURRENT key path; got:\n%s", got)
	}
	if !strings.Contains(strings.ToLower(got), "current key") {
		t.Errorf("the tested key must be labelled as the CURRENT key; got:\n%s", got)
	}
	if !strings.Contains(got, rotateDryRunCaveat) {
		t.Errorf("output must carry the frozen caveat sentence; got:\n%s", got)
	}
	if strings.Contains(got, "will reach") || strings.Contains(got, "proves the rotation") || strings.Contains(got, "replacement is") {
		t.Errorf("output must make no claim about the replacement key's reachability; got:\n%s", got)
	}
}

// --- delete's dry-run contract agrees with lifecycleStages (review R2-02) ----

// TestIdentityDeleteDryRunNoTestStageAndZeroTesterSeam proves this plan's
// delete dry-run contract and plan 05-07's lifecycleStages agree: delete's row
// contains no test stage (read from the table, not restated), and a recording
// tester seam counts ZERO invocations for BOTH a delete dry run and a full
// delete.
func TestIdentityDeleteDryRunNoTestStageAndZeroTesterSeam(t *testing.T) {
	seamGuard(t)
	deleteRow := lifecycleStages["delete"]
	if deleteRow == nil {
		t.Fatal("lifecycleStages has no delete row")
	}
	for _, stage := range deleteRow {
		if stage == "test" {
			t.Fatalf("lifecycleStages[delete] = %v must not contain a test stage", deleteRow)
		}
	}

	home := t.TempDir()
	t.Setenv("HOME", home)
	seedDeleteFixture(t, home, "work")

	testerCalls := 0
	cliConnectivityTest = func(_ *realBackend, _ string) tester.Result {
		testerCalls++
		return tester.Result{Outcome: tester.PASS}
	}

	cmd, out, _ := cliTestCmd()
	if err := runIdentityDelete(cmd, "work", identityDeleteFlags{GitOnly: true, DryRun: true}, false, false); err != nil {
		t.Fatalf("delete --dry-run: %v", err)
	}
	if !strings.Contains(out.String(), "would delete") {
		t.Errorf("the delete plan was not printed; output:\n%s", out.String())
	}
	if !strings.Contains(out.String(), "Repo remotes using git@") {
		t.Errorf("the disclaimer must be printed; output:\n%s", out.String())
	}
	if testerCalls != 0 {
		t.Errorf("delete dry run invoked the tester %d times, want 0", testerCalls)
	}

	if err := runIdentityDelete(cmd, "work", identityDeleteFlags{GitOnly: true, Yes: true}, false, false); err != nil {
		t.Fatalf("delete --git-only --yes: %v", err)
	}
	if testerCalls != 0 {
		t.Errorf("full delete invoked the tester %d times, want 0", testerCalls)
	}
}

// --- recording lifecycle doubles: exactly once, no file effect --------------

func TestIdentityDeleteRecordingDoubleInvokedExactlyOnce(t *testing.T) {
	seamGuard(t)
	home := t.TempDir()
	t.Setenv("HOME", home)
	seedDeleteFixture(t, home, "work")
	before := homeFileListing(t, home)

	var calls int
	cliDeleteInto = func(_ *realBackend, _ string, _ identity.DeleteScope, _ lifecyclePolicy) (lifecycleResult, error) {
		calls++
		return lifecycleResult{}, nil
	}

	cmd, _, _ := cliTestCmd()
	if err := runIdentityDelete(cmd, "work", identityDeleteFlags{GitOnly: true, Yes: true}, false, false); err != nil {
		t.Fatalf("delete --git-only --yes: %v", err)
	}
	if calls != 1 {
		t.Errorf("runDelete invoked %d times, want exactly 1", calls)
	}
	after := homeFileListing(t, home)
	if strings.Join(before, "\n") != strings.Join(after, "\n") {
		t.Errorf("the handler must produce no file effect of its own:\nbefore: %v\nafter:  %v", before, after)
	}
}

func TestIdentityRotateAndRepairRecordingDoubleInvokedExactlyOnce(t *testing.T) {
	for _, verb := range []string{"rotate", "new-key"} {
		verb := verb
		t.Run(verb, func(t *testing.T) {
			seamGuard(t)
			home := t.TempDir()
			t.Setenv("HOME", home)
			seedDeleteFixture(t, home, "work")
			before := homeFileListing(t, home)

			var calls int
			cliRotateInto = func(_ *realBackend, _ string, _ lifecyclePolicy) (lifecycleResult, error) {
				calls++
				return lifecycleResult{ReTest: tester.Result{Outcome: tester.PASS}}, nil
			}
			cliRepairInto = func(_ *realBackend, _ string, _ lifecyclePolicy) (lifecycleResult, error) {
				calls++
				return lifecycleResult{ReTest: tester.Result{Outcome: tester.PASS}}, nil
			}

			cmd, _, _ := cliTestCmd()
			if err := runIdentityKeyVerb(cmd, "work", verb, identityKeyFlags{Yes: true}, false, false); err != nil {
				t.Fatalf("%s --yes: %v", verb, err)
			}
			if calls != 1 {
				t.Errorf("lifecycle invoked %d times, want exactly 1", calls)
			}
			after := homeFileListing(t, home)
			if strings.Join(before, "\n") != strings.Join(after, "\n") {
				t.Errorf("the handler must produce no file effect of its own:\nbefore: %v\nafter:  %v", before, after)
			}
		})
	}
}

func TestIdentityCreateRecordingDoubleInvokedExactlyOnce(t *testing.T) {
	seamGuard(t)
	home := t.TempDir()
	t.Setenv("HOME", home)
	before := homeFileListing(t, home)

	cliPreWriteGate = func(b *realBackend, in identity.CreateInput, _ tuikit.DemoIdentity) (tuikit.WizardStageMsg, tuikit.WizardStageMsg, error) {
		b.recordOutcomeFor(1, tuikit.TestOutcomePass, in)
		b.recordOutcomeFor(2, tuikit.TestOutcomePass, in)
		return passStageMsg(tuikit.TestOutcomePass), passStageMsg(tuikit.TestOutcomePass), nil
	}
	var calls int
	commitCreateInto = func(_ *realBackend, _ identity.CreateInput, _ identity.StagedKey, _ tuikit.DemoIdentity) ([]string, error) {
		calls++
		return []string{"~/.gitconfig.bak.1", "~/.ssh/config.bak.1"}, nil
	}

	cmd, out, _ := cliTestCmd()
	err := runIdentityCreate(cmd, identityCreateFlags{
		Name: "work", Provider: "github.com", GitName: "Work Worker", GitEmail: "work@example.com", Yes: true,
	}, false, false)
	if err != nil {
		t.Fatalf("identity create --yes: %v", err)
	}
	if calls != 1 {
		t.Errorf("create lifecycle invoked %d times, want exactly 1", calls)
	}
	if !strings.Contains(out.String(), "created \"work\"") {
		t.Errorf("expected a created receipt; output:\n%s", out.String())
	}
	after := homeFileListing(t, home)
	if strings.Join(before, "\n") != strings.Join(after, "\n") {
		t.Errorf("the handler must produce no file effect under HOME:\nbefore: %v\nafter:  %v", before, after)
	}
}

func TestIdentityCloneRecordingDoubleInvokedExactlyOnce(t *testing.T) {
	seamGuard(t)
	home := t.TempDir()
	t.Setenv("HOME", home)
	seedDeleteFixture(t, home, "work")
	before := homeFileListing(t, home)

	cliPreWriteGate = func(b *realBackend, in identity.CreateInput, _ tuikit.DemoIdentity) (tuikit.WizardStageMsg, tuikit.WizardStageMsg, error) {
		b.recordOutcomeFor(1, tuikit.TestOutcomePass, in)
		b.recordOutcomeFor(2, tuikit.TestOutcomePass, in)
		return passStageMsg(tuikit.TestOutcomePass), passStageMsg(tuikit.TestOutcomePass), nil
	}
	var calls int
	commitCreateInto = func(_ *realBackend, _ identity.CreateInput, _ identity.StagedKey, _ tuikit.DemoIdentity) ([]string, error) {
		calls++
		return nil, nil
	}

	cmd, out, _ := cliTestCmd()
	err := runIdentityClone(cmd, "work", identityCloneFlags{Name: "work-clone", Yes: true}, false, false)
	if err != nil {
		t.Fatalf("identity clone work --name work-clone --yes: %v", err)
	}
	if calls != 1 {
		t.Errorf("clone lifecycle invoked %d times, want exactly 1", calls)
	}
	if !strings.Contains(out.String(), "created \"work-clone\"") {
		t.Errorf("expected a created receipt; output:\n%s", out.String())
	}
	after := homeFileListing(t, home)
	if strings.Join(before, "\n") != strings.Join(after, "\n") {
		t.Errorf("the handler must produce no file effect of its own:\nbefore: %v\nafter:  %v", before, after)
	}
}

// TestCloneCeremonyInputsFingerprintsMatchTestStage proves the store gate a
// headless clone consults is the same fingerprint TestStage1/2 record: empty
// Algo and a display-form reuse path would pass both stages and still be
// refused as stale.
func TestCloneCeremonyInputsFingerprintsMatchTestStage(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	seedDeleteFixture(t, home, "work")
	b := newBackendForHome(home)
	in, id, err := cloneCeremonyInputs(b, "work", "work-clone", true)
	if err != nil {
		t.Fatalf("cloneCeremonyInputs: %v", err)
	}
	if in.Algo == "" {
		t.Fatal("clone CreateInput.Algo is empty; TestStage defaults it to ed25519 and the store gate treats the pair as stale")
	}
	if in.Alias != "work-clone.github.com" {
		t.Fatalf("clone alias = %q, want work-clone.github.com (FQDN, not the reconstructed short token)", in.Alias)
	}
	staged := b.createInputFromSpec(tuikit.CreateSpec{
		Identity:     id.Name,
		Alias:        in.Alias,
		Hostname:     in.Hostname,
		Port:         fmt.Sprintf("%d", in.Port),
		ReuseKeyPath: in.ReuseKeyPath,
		Algorithm:    id.Algorithm,
		Provider:     in.Provider,
	})
	if got, want := specFingerprint(in), specFingerprint(staged); got != want {
		t.Fatalf("clone CreateInput fingerprint %q != TestStage reconstruction %q", got, want)
	}
}

// TestCreateInputFromCreateFlagsForceSSHDefaultsOffAndFlagEnables is the
// WR-06 regression: createInputFromCreateFlags hardcoded ForceSSH: true
// unconditionally, so every headless `identity create` wrote
// `[url "git@<provider>:"] insteadOf = https://<provider>/` into
// ~/.gitconfig — a MACHINE-GLOBAL rewrite of every HTTPS clone URL for that
// provider, for every repository, including ones unrelated to gitid. The
// TUI exposes this as a user-visible toggle; the CLI must default it off
// and gate it behind an explicit --force-ssh flag.
func TestCreateInputFromCreateFlagsForceSSHDefaultsOffAndFlagEnables(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	b := newBackendForHome(home)

	_, id, err := createInputFromCreateFlags(b, identityCreateFlags{
		Name: "work", Provider: "github.com", GitName: "Work User", GitEmail: "work@example.com",
	})
	if err != nil {
		t.Fatalf("createInputFromCreateFlags: %v", err)
	}
	if id.ForceSSH {
		t.Error("WR-06: identity create without --force-ssh must default ForceSSH to false")
	}

	_, id2, err := createInputFromCreateFlags(b, identityCreateFlags{
		Name: "work", Provider: "github.com", GitName: "Work User", GitEmail: "work@example.com", ForceSSH: true,
	})
	if err != nil {
		t.Fatalf("createInputFromCreateFlags --force-ssh: %v", err)
	}
	if !id2.ForceSSH {
		t.Error("WR-06: identity create --force-ssh must set ForceSSH true")
	}
}

// TestCloneCeremonyInputsGitDirMatchesClonePrefillDerivation is the WR-10
// regression: cloneCeremonyInputs hardcoded its own GitDir literal
// ("~/git/"+in.Name+"/") instead of deriving it via gitDirFromMatches(in.
// Matches) the way realBackend.ClonePrefill (the TUI wizard's prefill,
// consulted by this same verb's resolvePrefilledTUI branch a few lines
// above) already does — a duplicated derivation that a future change to
// either side could silently diverge. The two must now derive from the
// SAME formula and therefore agree. (Investigation note: with
// identity.DeriveCloneInput's current kind-only Matches rebuild — R-12 —
// both derivations happen to already coincide on every constructible
// input, so this guards against FUTURE drift between the two call sites
// rather than reproducing a live divergent value today.)
func TestCloneCeremonyInputsGitDirMatchesClonePrefillDerivation(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	seedDeleteFixture(t, home, "work")
	b := newBackendForHome(home)

	_, id, err := cloneCeremonyInputs(b, "work", "work-clone", true)
	if err != nil {
		t.Fatalf("cloneCeremonyInputs: %v", err)
	}
	prefill, perr := b.ClonePrefill("work", "work-clone", true)
	if perr != nil {
		t.Fatalf("ClonePrefill: %v", perr)
	}
	if id.GitDir != prefill.GitDir {
		t.Errorf("WR-10: headless GitDir = %q, wizard prefill GitDir = %q — the two clone paths must derive the SAME includeIf gitdir from the SAME identity.DeriveCloneInput matches, never a second, independently hardcoded default", id.GitDir, prefill.GitDir)
	}
}

// TestCloneCeremonyInputsPublicKeyPathNeverBarePubSuffix is the WR-11
// regression: cloneCeremonyInputs' DemoIdentity{} struct literal set
// PublicKeyPath: in.ReuseKeyPath + ".pub" eagerly, then the if/else block a
// few lines below unconditionally reassigned it — dead when ReuseKeyPath is
// non-empty (immediately overwritten with the identical value), and a live
// trap when it is empty (the literal alone would have produced the bare
// string ".pub", the exact defect WR-14 in identities.go was written to
// eliminate) — harmless only because the reassignment always runs. This
// guards BOTH branches directly so a future reordering that drops the
// explicit reassignment fails loudly instead of silently regressing to
// ".pub".
func TestCloneCeremonyInputsPublicKeyPathNeverBarePubSuffix(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	seedDeleteFixture(t, home, "work")
	b := newBackendForHome(home)

	_, idReuse, err := cloneCeremonyInputs(b, "work", "work-clone", true)
	if err != nil {
		t.Fatalf("cloneCeremonyInputs(reuse): %v", err)
	}
	if idReuse.PublicKeyPath == ".pub" || idReuse.PublicKeyPath == "" {
		t.Errorf("WR-11: reuse-key clone PublicKeyPath = %q, must not be the bare \".pub\" suffix", idReuse.PublicKeyPath)
	}

	_, idGen, err := cloneCeremonyInputs(b, "work", "work-clone2", false)
	if err != nil {
		t.Fatalf("cloneCeremonyInputs(generated): %v", err)
	}
	if want := "~/.ssh/id_ed25519_work-clone2.pub"; idGen.PublicKeyPath != want {
		t.Errorf("WR-11: generated-key clone PublicKeyPath = %q, want %q", idGen.PublicKeyPath, want)
	}
}

// TestCloneCeremonyInputsMirrorsSourceForceSSH is WR-06's clone half:
// cloneCeremonyInputs hardcoded ForceSSH: true unconditionally, contradicting
// D-15's "copy the author fields, re-derive the rest" — a clone must not
// silently switch on a machine-global rewrite the source never had. The
// clone's ForceSSH must mirror src.ForceSSH, not force it on.
func TestCloneCeremonyInputsMirrorsSourceForceSSH(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	seedDeleteFixture(t, home, "work") // no provider-rewrite block seeded -> src.ForceSSH == false
	b := newBackendForHome(home)

	_, id, err := cloneCeremonyInputs(b, "work", "work-clone", true)
	if err != nil {
		t.Fatalf("cloneCeremonyInputs: %v", err)
	}
	if id.ForceSSH {
		t.Error("WR-06: cloning a source without the provider rewrite must not force it on for the clone")
	}
}

// --- the post-write re-test drives the exit code (D-02) ----------------------

// TestIdentityKeyVerbReTestDrivesExitCode proves a failing post-write re-test
// exits non-zero naming the outcome, while a reachable-not-uploaded landing
// state exits ZERO (an accepted landing state for a fresh key).
func TestIdentityKeyVerbReTestDrivesExitCode(t *testing.T) {
	for _, verb := range []string{"rotate", "new-key"} {
		verb := verb
		t.Run(verb+"/retest-failure", func(t *testing.T) {
			seamGuard(t)
			home := t.TempDir()
			t.Setenv("HOME", home)
			cliRotateInto = func(_ *realBackend, _ string, _ lifecyclePolicy) (lifecycleResult, error) {
				return lifecycleResult{ReTest: tester.Result{Outcome: tester.Failure}}, nil
			}
			cliRepairInto = func(_ *realBackend, _ string, _ lifecyclePolicy) (lifecycleResult, error) {
				return lifecycleResult{ReTest: tester.Result{Outcome: tester.Failure}}, nil
			}

			cmd, _, _ := cliTestCmd()
			err := runIdentityKeyVerb(cmd, "ghost", verb, identityKeyFlags{Yes: true}, false, false)
			if err == nil {
				t.Fatalf("a failing post-write re-test must exit non-zero")
			}
			if !strings.Contains(err.Error(), "re-test failed") || !strings.Contains(err.Error(), "failure") {
				t.Errorf("the error must name the failing outcome; got %q", err.Error())
			}
		})
		t.Run(verb+"/reachable-not-uploaded-zero", func(t *testing.T) {
			seamGuard(t)
			home := t.TempDir()
			t.Setenv("HOME", home)
			cliRotateInto = func(_ *realBackend, _ string, _ lifecyclePolicy) (lifecycleResult, error) {
				return lifecycleResult{ReTest: tester.Result{Outcome: tester.ReachableNotUploaded}}, nil
			}
			cliRepairInto = func(_ *realBackend, _ string, _ lifecyclePolicy) (lifecycleResult, error) {
				return lifecycleResult{ReTest: tester.Result{Outcome: tester.ReachableNotUploaded}}, nil
			}

			cmd, _, _ := cliTestCmd()
			if err := runIdentityKeyVerb(cmd, "ghost", verb, identityKeyFlags{Yes: true}, false, false); err != nil {
				t.Fatalf("a reachable-not-uploaded re-test is an accepted landing state and must exit zero: %v", err)
			}
		})
	}
}

// --- source-level: confirmationAlreadyObtained is unreachable from CLI code --

// TestIdentityCLICannotReachConfirmationAlreadyObtained asserts the identifier
// `confirmationAlreadyObtained` appears ZERO times in the CLI handler source
// files — the TUI's authorization value is reserved for a layer that rendered
// a confirm screen, and a future CLI handler cannot borrow it to silence a
// prompt (review R2-03).
func TestIdentityCLICannotReachConfirmationAlreadyObtained(t *testing.T) {
	root := testRepoRoot(t)
	for _, name := range []string{
		"identity.go", "identity_read.go", "identity_delete.go",
		"identity_create.go", "identity_clone.go", "identity_key.go",
		"ssh.go",
	} {
		data, err := os.ReadFile(filepath.Join(root, "cmd", "gitid", name)) //nolint:gosec // fixed repository-relative source path (G304)
		if err != nil {
			t.Fatalf("reading %s: %v", name, err)
		}
		if strings.Contains(string(data), "confirmationAlreadyObtained") {
			t.Errorf("%s references confirmationAlreadyObtained — a CLI path must never use the TUI's authorization value (review R2-03)", name)
		}
	}
}

// --- per-verb dry-run tests (review R12-DR) ----------------------------------

func TestIdentityCreateDryRunPrintsGateOutcomesAndPreviews(t *testing.T) {
	seamGuard(t)
	home := t.TempDir()
	t.Setenv("HOME", home)
	before := homeFileListing(t, home)

	cliPreWriteGate = func(_ *realBackend, _ identity.CreateInput, _ tuikit.DemoIdentity) (tuikit.WizardStageMsg, tuikit.WizardStageMsg, error) {
		return passStageMsg(tuikit.TestOutcomePass), passStageMsg(tuikit.TestOutcomeReachableNotUploaded), nil
	}

	cmd, out, _ := cliTestCmd()
	err := runIdentityCreate(cmd, identityCreateFlags{
		Name: "work", Provider: "github.com", GitName: "Work Worker", GitEmail: "work@example.com", DryRun: true,
	}, false, false)
	if err != nil {
		t.Fatalf("identity create --dry-run: %v", err)
	}
	got := out.String()
	for _, want := range []string{
		"would create \"work\"",
		"stage 1 (key direct): PASS",
		"stage 2 (alias via staged config): reachable-not-uploaded",
		"ssh config block",
		"fragment (~/.gitconfig.d/work)",
		"includeIf block",
		"allowed_signers",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("dry-run output missing %q; got:\n%s", want, got)
		}
	}
	after := homeFileListing(t, home)
	if strings.Join(before, "\n") != strings.Join(after, "\n") {
		t.Errorf("create --dry-run must not write anything under ~/.ssh or ~/.gitconfig:\nbefore: %v\nafter:  %v", before, after)
	}
}

func TestIdentityCloneDryRunPrintsGateOutcomesAndPreviews(t *testing.T) {
	seamGuard(t)
	home := t.TempDir()
	t.Setenv("HOME", home)
	seedDeleteFixture(t, home, "work")
	before := homeFileListing(t, home)

	cliPreWriteGate = func(_ *realBackend, _ identity.CreateInput, _ tuikit.DemoIdentity) (tuikit.WizardStageMsg, tuikit.WizardStageMsg, error) {
		return passStageMsg(tuikit.TestOutcomePass), passStageMsg(tuikit.TestOutcomePass), nil
	}

	cmd, out, _ := cliTestCmd()
	err := runIdentityClone(cmd, "work", identityCloneFlags{Name: "work-clone", DryRun: true}, false, false)
	if err != nil {
		t.Fatalf("identity clone --dry-run: %v", err)
	}
	got := out.String()
	for _, want := range []string{
		"would create \"work-clone\"",
		"stage 1 (key direct): PASS",
		"stage 2 (alias via staged config): PASS",
		"ssh config block",
		"fragment (~/.gitconfig.d/work-clone)",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("dry-run output missing %q; got:\n%s", want, got)
		}
	}
	after := homeFileListing(t, home)
	if strings.Join(before, "\n") != strings.Join(after, "\n") {
		t.Errorf("clone --dry-run must not write anything under ~/.ssh or ~/.gitconfig:\nbefore: %v\nafter:  %v", before, after)
	}
}

// TestIdentityReadVerbsRejectDryRun asserts `list --dry-run` and `show
// --dry-run` are usage errors — reads never write.
func TestIdentityReadVerbsRejectDryRun(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	seedDeleteFixture(t, home, "work")

	cmd := newVerbCmd(newIdentityListVerb())
	cmd.SetOut(ioDiscard{})
	cmd.SetArgs([]string{"--dry-run"})
	if err := cmd.Execute(); err == nil {
		t.Error("list --dry-run must be a usage error")
	} else if !strings.Contains(err.Error(), "--dry-run") {
		t.Errorf("list --dry-run error = %q, want it to name --dry-run", err.Error())
	}

	cmd2 := newVerbCmd(newIdentityShowVerb())
	cmd2.SetOut(ioDiscard{})
	cmd2.SetArgs([]string{"work", "--dry-run"})
	if err := cmd2.Execute(); err == nil {
		t.Error("show --dry-run must be a usage error")
	} else if !strings.Contains(err.Error(), "--dry-run") {
		t.Errorf("show --dry-run error = %q, want it to name --dry-run", err.Error())
	}
}

// --- identity-of-sequence: CLI and TUI run the SAME stages (D-02) ------------

// TestIdentityKeyVerbStageSequenceMatchesTUI proves the D-02 no-behavioral-fork
// claim directly for rotate and new-key: the stage sequence recorded by a
// CLI-driven runRotate/runRepair is IDENTICAL to the sequence recorded by the
// TUI-driven CommitRotate/CommitNewKey, and both equal lifecycleStages.
func TestIdentityKeyVerbStageSequenceMatchesTUI(t *testing.T) {
	for _, verb := range []string{"rotate", "new-key"} {
		verb := verb
		t.Run(verb, func(t *testing.T) {
			seamGuard(t)
			homeCLI := t.TempDir()
			t.Setenv("HOME", homeCLI)
			seedDeleteFixture(t, homeCLI, "work")

			var cliStages []string
			cliRotateInto = func(b *realBackend, name string, p lifecyclePolicy) (lifecycleResult, error) {
				b.deps = realHermeticDeps(b.deps)
				p.Stages = func(s string) { cliStages = append(cliStages, s) }
				return b.runRotate(name, p)
			}
			cliRepairInto = func(b *realBackend, name string, p lifecyclePolicy) (lifecycleResult, error) {
				b.deps = realHermeticDeps(b.deps)
				p.Stages = func(s string) { cliStages = append(cliStages, s) }
				return b.runRepair(name, p)
			}

			cmd, _, _ := cliTestCmd()
			if err := runIdentityKeyVerb(cmd, "work", verb, identityKeyFlags{Yes: true}, false, false); err != nil {
				t.Fatalf("CLI %s --yes: %v", verb, err)
			}

			homeTUI := t.TempDir()
			seedDeleteFixture(t, homeTUI, "work")
			b := newBackendForHome(homeTUI)
			var tuiStages []string
			commitRotateInto = func(bb *realBackend, name string, p lifecyclePolicy) (lifecycleResult, error) {
				bb.deps = realHermeticDeps(bb.deps)
				p.Stages = func(s string) { tuiStages = append(tuiStages, s) }
				return bb.runRotate(name, p)
			}
			commitRepairInto = func(bb *realBackend, name string, p lifecyclePolicy) (lifecycleResult, error) {
				bb.deps = realHermeticDeps(bb.deps)
				p.Stages = func(s string) { tuiStages = append(tuiStages, s) }
				return bb.runRepair(name, p)
			}
			var msg interface{}
			if verb == "new-key" {
				msg = b.CommitNewKey("work")()
			} else {
				msg = b.CommitRotate("work")()
			}
			if _, ok := msg.(tuikit.KeyCommitMsg); !ok {
				t.Fatalf("TUI commit delivered %T, want KeyCommitMsg", msg)
			}

			want := append([]string(nil), lifecycleStages[stageTableKey(verb)]...)
			if strings.Join(cliStages, ",") != strings.Join(want, ",") {
				t.Errorf("CLI %s stages = %v, want %v", verb, cliStages, want)
			}
			if strings.Join(tuiStages, ",") != strings.Join(want, ",") {
				t.Errorf("TUI %s stages = %v, want %v", verb, tuiStages, want)
			}
			if strings.Join(cliStages, ",") != strings.Join(tuiStages, ",") {
				t.Errorf("CLI and TUI %s stage sequences differ:\nCLI: %v\nTUI: %v", verb, cliStages, tuiStages)
			}
		})
	}
}

// stageTableKey maps a CLI verb name onto its lifecycleStages row key
// (lifecycle.go's table is keyed by lifecycle function: rotate/repair/delete).
func stageTableKey(verb string) string {
	if verb == "new-key" {
		return "repair"
	}
	return verb
}

// TestIdentityDeleteStageSequenceMatchesTUI is the delete half of the
// identity-of-sequence proof. The TUI's CommitDelete is a direct thin adapter
// — `res, err := b.runDelete(name, deleteScope, lifecyclePolicy{Confirm:
// confirmationAlreadyObtained})` — so the TUI half records the sequence by
// invoking runDelete with the EXACT policy CommitDelete uses.
func TestIdentityDeleteStageSequenceMatchesTUI(t *testing.T) {
	seamGuard(t)
	homeCLI := t.TempDir()
	t.Setenv("HOME", homeCLI)
	seedDeleteFixture(t, homeCLI, "work")

	var cliStages []string
	cliDeleteInto = func(b *realBackend, name string, scope identity.DeleteScope, p lifecyclePolicy) (lifecycleResult, error) {
		p.Stages = func(s string) { cliStages = append(cliStages, s) }
		return b.runDelete(name, scope, p)
	}

	cmd, _, _ := cliTestCmd()
	if err := runIdentityDelete(cmd, "work", identityDeleteFlags{GitOnly: true, Yes: true}, false, false); err != nil {
		t.Fatalf("CLI delete --git-only --yes: %v", err)
	}

	homeTUI := t.TempDir()
	seedDeleteFixture(t, homeTUI, "work")
	b := newBackendForHome(homeTUI)
	var tuiStages []string
	if _, err := b.runDelete("work", identity.DeleteScopeGitOnly, lifecyclePolicy{
		Confirm: confirmationAlreadyObtained,
		Stages:  func(s string) { tuiStages = append(tuiStages, s) },
	}); err != nil {
		t.Fatalf("TUI delete (runDelete with CommitDelete's exact policy): %v", err)
	}

	want := append([]string(nil), lifecycleStages["delete"]...)
	if strings.Join(cliStages, ",") != strings.Join(want, ",") {
		t.Errorf("CLI delete stages = %v, want %v", cliStages, want)
	}
	if strings.Join(tuiStages, ",") != strings.Join(want, ",") {
		t.Errorf("TUI delete stages = %v, want %v", tuiStages, want)
	}
	if strings.Join(cliStages, ",") != strings.Join(tuiStages, ",") {
		t.Errorf("CLI and TUI delete stage sequences differ:\nCLI: %v\nTUI: %v", cliStages, tuiStages)
	}
}

// --- misc handler behaviors -------------------------------------------------

func TestIdentityCloneMissingNameNonInteractiveError(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	seedDeleteFixture(t, home, "work")

	cmd, _, _ := cliTestCmd()
	err := runIdentityClone(cmd, "work", identityCloneFlags{}, false, false)
	if err == nil {
		t.Fatal("clone without --name and without a terminal must exit non-zero")
	}
	if !strings.Contains(err.Error(), "name") {
		t.Errorf("error = %q, want it to name the name flag", err.Error())
	}
}

// ---------------------------------------------------------------------------
// Task 2 — the requirement-keyed parity matrix (D-04).
//
// The matrix in docs/cli-parity-matrix.md is the machine-checked, requirement-
// keyed outcome-to-command map D-04 requires. These tests read the file (via
// testRepoRoot, like every repository-locating test here), parse BOTH tables,
// walk newRootCmd(), and assert parity in both directions across EVERY noun
// group — never only `identity`:
//
//   - all three tooling exclusions (completion / help / debug) are named in the
//     matrix's header and never required to appear in a row;
//   - the command-path resolution, deferred-noun phase agreement, tree
//     coverage, and dry-run contract-table checks below are each backed by a
//     negative control so they cannot pass vacuously.
// ---------------------------------------------------------------------------

// matrixRow is one parsed row of the requirement-keyed outcome table.
type matrixRow struct {
	requirement string
	outcome     string
	command     string
	flags       string
	status      string
	notes       string
}

// splitCells splits a `| a | b |` table row into its trimmed cell values.
func splitCells(line string) []string {
	s := strings.TrimPrefix(strings.TrimSpace(line), "|")
	s = strings.TrimSuffix(s, "|")
	parts := strings.Split(s, "|")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		out = append(out, strings.TrimSpace(p))
	}
	return out
}

// parseParityMatrix reads docs/cli-parity-matrix.md and parses both tables.
// It returns the outcome rows and the comma-separated verb names from the
// dry-run contract table's Verb column.
func parseParityMatrix(t *testing.T) (rows []matrixRow, dryRunVerbs []string) {
	t.Helper()
	path := filepath.Join(testRepoRoot(t), "docs", "cli-parity-matrix.md")
	raw, err := os.ReadFile(path) //nolint:gosec // fixed repository-relative matrix path (G304)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	section := ""
	for _, line := range strings.Split(string(raw), "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "## ") {
			section = trimmed
			continue
		}
		if !strings.HasPrefix(trimmed, "| ") || strings.HasPrefix(trimmed, "|---") {
			continue
		}
		switch section {
		case "## Requirement-keyed outcome matrix":
			cells := splitCells(trimmed)
			if len(cells) < 6 || strings.EqualFold(cells[0], "Requirement") {
				continue
			}
			rows = append(rows, matrixRow{
				requirement: cells[0],
				outcome:     cells[1],
				command:     cells[2],
				flags:       cells[3],
				status:      cells[4],
				notes:       cells[5],
			})
		case "## The per-verb `--dry-run` contract (R12-DR)":
			cells := splitCells(trimmed)
			if len(cells) < 1 || strings.EqualFold(cells[0], "Verb") {
				continue
			}
			dryRunVerbs = append(dryRunVerbs, cells[0])
		}
	}
	return rows, dryRunVerbs
}

// namedCommandPaths returns every fully-qualified command path named by a
// row's Command cell — the noun form and the flat alias form separated by
// " / ". Backticks around the inline-code literals, positional-arg tokens
// like <name>, and the deferred "…" marker are dropped.
func namedCommandPaths(row matrixRow) []string {
	var paths []string
	for _, form := range strings.Split(row.command, "/") {
		var toks []string
		for _, tok := range strings.Fields(strings.ReplaceAll(form, "`", "")) {
			if tok == "…" || (strings.HasPrefix(tok, "<") && strings.HasSuffix(tok, ">")) {
				continue
			}
			toks = append(toks, tok)
		}
		paths = append(paths, strings.Join(toks, " "))
	}
	return paths
}

// reservedNoun reports whether cmd is a reserved placeholder noun (ssh/git/
// health/fix) that "arrives in a later phase" — it is NOT a real writer, so a
// shipped row must never resolve to it.
func reservedNoun(cmd *cobra.Command) bool {
	switch cmd.Name() {
	case "ssh", "git", "health", "fix":
		return len(cmd.Commands()) == 0 && strings.Contains(cmd.Short, "arrives in")
	}
	return false
}

// parityToolingExcluded reports whether a fully-qualified path is tooling
// (completion / help / debug) that the matrix explicitly documents as never
// required to appear in a row.
func parityToolingExcluded(path string) bool {
	return strings.HasPrefix(path, "gitid completion ") ||
		strings.HasPrefix(path, "gitid completion") ||
		strings.HasPrefix(path, "gitid help ") ||
		strings.HasPrefix(path, "gitid help") ||
		strings.HasPrefix(path, "gitid debug")
}

// checkParityMatrix verifies the matrix against the built command tree in
// BOTH directions and returns a list of problems (empty when consistent):
//
//   - every shipped row's command path resolves to a runnable, non-reserved
//     command in the tree;
//   - every deferred Phase N row's noun resolves to a reserved-noun command
//     whose phase error names the same Phase N;
//   - every runnable command in the tree outside the tooling exclusions is
//     named by at least one row.
//
// It is intentionally a plain function (not a test) so the three negative
// controls can feed it deliberately-incoherent inputs and prove it is not
// vacuous in all three directions.
func checkParityMatrix(rows []matrixRow, root *cobra.Command) []string {
	var problems []string
	named := map[string]bool{}

	for _, row := range rows {
		for _, path := range namedCommandPaths(row) {
			named[path] = true
			toks := strings.Fields(path)
			if len(toks) < 2 {
				problems = append(problems, fmt.Sprintf("row %q has malformed command %q", row.outcome, path))
				continue
			}
			cmd, _, err := root.Find(toks[1:])
			if err != nil {
				problems = append(problems, fmt.Sprintf("row %q names unresolvable command %q: %v", row.outcome, path, err))
				continue
			}
			if cmd.CommandPath() != path {
				problems = append(problems, fmt.Sprintf("row %q names command %q which resolves to %q", row.outcome, path, cmd.CommandPath()))
				continue
			}
			switch {
			case strings.TrimSpace(row.status) == "shipped":
				if cmd.RunE == nil || reservedNoun(cmd) {
					problems = append(problems, fmt.Sprintf("row %q (shipped) names %q which is not a real writer (reserved noun or no RunE)", row.outcome, path))
				}
			case strings.HasPrefix(strings.TrimSpace(row.status), "deferred"):
				phasePart := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(row.status), "deferred"))
				if phasePart == "" {
					problems = append(problems, fmt.Sprintf("row %q deferred status %q names no phase", row.outcome, row.status))
					continue
				}
				if cmd.RunE == nil || !reservedNoun(cmd) {
					problems = append(problems, fmt.Sprintf("row %q (deferred) names %q which is not a reserved placeholder noun", row.outcome, path))
					continue
				}
				rerr := cmd.RunE(cmd, nil)
				if rerr == nil || !strings.Contains(rerr.Error(), phasePart) {
					problems = append(problems, fmt.Sprintf("row %q (deferred) noun %q error does not name phase %q", row.outcome, path, phasePart))
				}
			}
		}
	}

	var walk func(*cobra.Command)
	walk = func(c *cobra.Command) {
		if c.RunE != nil {
			path := c.CommandPath()
			if !parityToolingExcluded(path) && !named[path] {
				problems = append(problems, fmt.Sprintf("command %q is not named by any matrix row", path))
			}
		}
		for _, child := range c.Commands() {
			walk(child)
		}
	}
	walk(root)
	return problems
}

// TestParityMatrixResolvesAndCoversTree asserts the two-directional contract:
// every shipped row resolves to a real writer, every deferred Phase N row's
// reserved noun agrees on the phase, and every runnable command across ALL
// noun groups (identity, ssh, git, health, fix — not only identity) is named.
func TestParityMatrixResolvesAndCoversTree(t *testing.T) {
	rows, _ := parseParityMatrix(t)
	if len(rows) == 0 {
		t.Fatal("parsed zero outcome rows from the matrix")
	}
	root := newRootCmd()
	root.InitDefaultCompletionCmd()
	root.InitDefaultHelpCmd()

	problems := checkParityMatrix(rows, root)
	if len(problems) != 0 {
		t.Errorf("parity matrix inconsistent with tree (%d problems):\n%s", len(problems), strings.Join(problems, "\n"))
	}
}

// TestParityMatrixRequirementCoverage asserts the matrix keys on every
// requirement this plan claims to satisfy: each id appears in at least one row.
func TestParityMatrixRequirementCoverage(t *testing.T) {
	rows, _ := parseParityMatrix(t)
	required := []string{"MGR-04", "MGR-05", "MGR-06", "KEY-05", "KEY-07", "SHELL-03"}
	for _, id := range required {
		found := false
		for _, row := range rows {
			for _, token := range strings.Split(row.requirement, ",") {
				if strings.TrimSpace(token) == id {
					found = true
				}
			}
		}
		if !found {
			t.Errorf("no matrix row keys on requirement %s", id)
		}
	}
}

// TestParityMatrixDryRunContractTable asserts the second table lists a row for
// every write verb plus the reads list and show.
func TestParityMatrixDryRunContractTable(t *testing.T) {
	_, dryRunVerbs := parseParityMatrix(t)
	want := map[string]bool{"create": true, "clone": true, "rotate": true, "new-key": true, "delete": true, "list": true, "show": true}
	got := map[string]bool{}
	for _, cell := range dryRunVerbs {
		for _, tok := range strings.Split(cell, ",") {
			tok = strings.TrimSpace(tok)
			if i := strings.Index(tok, "("); i >= 0 {
				tok = strings.TrimSpace(tok[:i])
			}
			if tok != "" {
				got[tok] = true
			}
		}
	}
	for v := range want {
		if !got[v] {
			t.Errorf("dry-run contract table is missing a row for verb %q (have %v)", v, got)
		}
	}
	for v := range got {
		if !want[v] {
			t.Errorf("dry-run contract table lists unexpected verb %q", v)
		}
	}
}

// TestParityMatrixNegativeControlNonExistentCommand proves a shipped row
// naming a command that does not exist is reported.
func TestParityMatrixNegativeControlNonExistentCommand(t *testing.T) {
	rows := []matrixRow{{status: "shipped", outcome: "bogus outcome", command: "gitid identity bogus"}}
	problems := checkParityMatrix(rows, newRootCmd())
	if !strings.Contains(strings.Join(problems, "\n"), "bogus") {
		t.Errorf("checker did not report a shipped row naming a non-existent command; problems: %v", problems)
	}
}

// TestParityMatrixNegativeControlUnnamedTreeCommand proves a runnable tree
// command that no row names is reported.
func TestParityMatrixNegativeControlUnnamedTreeCommand(t *testing.T) {
	root := &cobra.Command{Use: "gitid"}
	root.AddCommand(&cobra.Command{
		Use:   "mystery",
		Short: "a fabricated command with a RunE no matrix row names",
		RunE:  func(*cobra.Command, []string) error { return nil },
	})
	problems := checkParityMatrix(nil, root)
	if !strings.Contains(strings.Join(problems, "\n"), "mystery") {
		t.Errorf("checker did not report an unnamed tree command; problems: %v", problems)
	}
}

// TestParityMatrixNegativeControlShippedReservedNoun proves a shipped row
// resolving only to a reserved placeholder noun (ssh/git/health/fix) is
// reported, so a shipped outcome can never hide behind a reserved error.
func TestParityMatrixNegativeControlShippedReservedNoun(t *testing.T) {
	rows := []matrixRow{{status: "shipped", outcome: "usurps a reserved noun", command: "gitid ssh"}}
	problems := checkParityMatrix(rows, newRootCmd())
	if !strings.Contains(strings.Join(problems, "\n"), "not a real writer") {
		t.Errorf("checker did not report a shipped row resolving to a reserved noun; problems: %v", problems)
	}
}
