package main

// identity_test.go covers the D-01 command tree (Task 3) and the D-03 read
// surface (Task 2): the shared identityVerb/newVerbCmd constructor, the
// reserved noun groups, and identity_read.go's record projection + output
// writers.

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/castocolina/gitid/internal/identity"
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
