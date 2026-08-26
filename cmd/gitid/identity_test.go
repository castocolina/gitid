package main

// identity_test.go covers the D-01 command tree (Task 3) and the D-03 read
// surface (Task 2): the shared identityVerb/newVerbCmd constructor, the
// reserved noun groups, and identity_read.go's record projection + output
// writers.

import (
	"bytes"
	"encoding/json"
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
		"ssh":    "Phase 6",
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
	return func() {
		commitCreateInto, cliPreWriteGate = oldCreate, oldGate
		cliRotateInto, cliRepairInto = oldRotate, oldRepair
		cliDeleteInto, cliConnectivityTest = oldDelete, oldTest
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
