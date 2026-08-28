package checks_test

import (
	"errors"
	"os"
	"testing"
	"time"

	"github.com/castocolina/gitid/internal/doctor"
	"github.com/castocolina/gitid/internal/doctor/checks"
	"github.com/castocolina/gitid/internal/globalgit"
	"github.com/castocolina/gitid/internal/globalssh"
	"github.com/castocolina/gitid/internal/identity"
	"github.com/castocolina/gitid/internal/sshconfig"
)

// cohFileInfo is a minimal os.FileInfo for coherence tests. It is separate from
// fakeFileInfo in perms_test.go to avoid redeclaration in the same test package.
type cohFileInfo struct{ mode os.FileMode }

func (c cohFileInfo) Name() string       { return "" }
func (c cohFileInfo) Size() int64        { return 0 }
func (c cohFileInfo) Mode() os.FileMode  { return c.mode }
func (c cohFileInfo) ModTime() time.Time { return time.Time{} }
func (c cohFileInfo) IsDir() bool        { return false }
func (c cohFileInfo) Sys() interface{}   { return nil }

// cohStat returns a Stat function where paths in presentPaths return a 0600
// FileInfo and all other paths return os.ErrNotExist.
func cohStat(presentPaths ...string) func(string) (os.FileInfo, error) {
	set := make(map[string]bool, len(presentPaths))
	for _, p := range presentPaths {
		set[p] = true
	}
	return func(path string) (os.FileInfo, error) {
		if set[path] {
			return cohFileInfo{mode: 0o600}, nil
		}
		return nil, os.ErrNotExist
	}
}

// signerLineFor builds a valid allowed_signers line for the given email.
func signerLineFor(email string) []byte {
	return []byte(email + ` namespaces="git" ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIFakeKey== comment` + "\n")
}

// makeAccount builds a minimal identity.Account for use in tests.
func makeAccount(name, alias, email, keyPath, fragPath, incomplete string) identity.Account {
	return identity.Account{
		Name:         name,
		Alias:        alias,
		GitEmail:     email,
		KeyPath:      keyPath,
		PubPath:      keyPath + ".pub",
		FragmentPath: fragPath,
		Incomplete:   incomplete,
	}
}

// TestCoherenceIdentityFileGone: account with KeyPath set, Stat→ErrNotExist → error finding.
func TestCoherenceIdentityFileGone(t *testing.T) {
	acct := makeAccount("work", "work.github.com", "work@example.com",
		"/home/u/.ssh/gitid_work", "/home/u/.gitconfig.d/work", "")
	d := doctor.Deps{
		Stat:       cohStat(), // no paths present
		Identities: []identity.Account{acct},
		ManagedHosts: map[string]sshconfig.SSHHostInfo{
			"work": {Alias: "work.github.com", IdentitiesOnly: true},
		},
		ReadFile: func(_ string) ([]byte, error) {
			return signerLineFor("work@example.com"), nil
		},
		RunGitConfigGet: func(_, _ string) (string, error) {
			return "ssh", nil // gpg.format = ssh
		},
		AllowedSignersPath: "/home/u/.ssh/allowed_signers",
	}

	findings := checks.CheckCoherence(d)

	if len(findings) == 0 {
		t.Fatal("expected at least one finding for missing IdentityFile, got none")
	}
	var found bool
	for _, f := range findings {
		if f.Family != doctor.FamilyCoherence {
			t.Errorf("finding family = %q, want %q", f.Family, doctor.FamilyCoherence)
		}
		if f.Severity == doctor.SeverityError && cohContains(f.Title, "does not exist") {
			found = true
			if f.Fix != nil {
				t.Error("IdentityFile missing finding must NOT carry a Fix descriptor")
			}
		}
	}
	if !found {
		t.Errorf("expected finding with 'does not exist' in title, got: %v", cohTitles(findings))
	}
}

// TestCoherenceFragmentGone: account.FragmentPath Stat→ErrNotExist → error finding.
func TestCoherenceFragmentGone(t *testing.T) {
	acct := makeAccount("work", "work.github.com", "work@example.com",
		"/home/u/.ssh/gitid_work", "/home/u/.gitconfig.d/work", "")
	d := doctor.Deps{
		Stat:       cohStat("/home/u/.ssh/gitid_work"), // key present, fragment missing
		Identities: []identity.Account{acct},
		ManagedHosts: map[string]sshconfig.SSHHostInfo{
			"work": {Alias: "work.github.com", IdentitiesOnly: true},
		},
		ReadFile: func(_ string) ([]byte, error) {
			return signerLineFor("work@example.com"), nil
		},
		RunGitConfigGet: func(_, _ string) (string, error) {
			return "ssh", nil
		},
		AllowedSignersPath: "/home/u/.ssh/allowed_signers",
	}

	findings := checks.CheckCoherence(d)

	var found bool
	for _, f := range findings {
		if f.Severity == doctor.SeverityError && cohContains(f.Title, "does not exist") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected 'does not exist' finding for fragment, got: %v", cohTitles(findings))
	}
}

// TestCoherenceIdentitiesOnly: managed Host with IdentitiesOnly==false → error + Fix.
func TestCoherenceIdentitiesOnly(t *testing.T) {
	acct := makeAccount("work", "work.github.com", "work@example.com",
		"/home/u/.ssh/gitid_work", "/home/u/.gitconfig.d/work", "")
	d := doctor.Deps{
		Stat:          cohStat("/home/u/.ssh/gitid_work", "/home/u/.gitconfig.d/work"),
		SSHConfigPath: "/home/u/.ssh/config", // required for Fix.Fn wiring
		Identities:    []identity.Account{acct},
		ManagedHosts: map[string]sshconfig.SSHHostInfo{
			"work": {Alias: "work.github.com", IdentitiesOnly: false}, // missing
		},
		ReadFile: func(_ string) ([]byte, error) {
			return signerLineFor("work@example.com"), nil
		},
		RunGitConfigGet: func(_, _ string) (string, error) {
			return "ssh", nil
		},
		AllowedSignersPath: "/home/u/.ssh/allowed_signers",
		// AddWiring required so Fix.Fn is non-nil (real coherence wiring).
		AddWiring: func(_, _, _ string) error { return nil },
	}

	findings := checks.CheckCoherence(d)

	var found bool
	for _, f := range findings {
		if f.Severity == doctor.SeverityError && cohContains(f.Title, "IdentitiesOnly yes missing") {
			found = true
			if f.Fix == nil {
				t.Error("IdentitiesOnly finding must carry a Fix descriptor ([fix] marker required)")
			}
		}
	}
	if !found {
		t.Errorf("expected 'IdentitiesOnly yes missing' finding, got: %v", cohTitles(findings))
	}
}

// TestCoherenceSignersLine: signing identity with no matching line in allowed_signers → error + Fix.
func TestCoherenceSignersLine(t *testing.T) {
	acct := makeAccount("personal", "personal.github.com", "personal@example.com",
		"/home/u/.ssh/gitid_personal", "/home/u/.gitconfig.d/personal", "")
	d := doctor.Deps{
		Stat:       cohStat("/home/u/.ssh/gitid_personal", "/home/u/.gitconfig.d/personal"),
		Identities: []identity.Account{acct},
		ManagedHosts: map[string]sshconfig.SSHHostInfo{
			"personal": {Alias: "personal.github.com", IdentitiesOnly: true},
		},
		// allowed_signers exists but has NO entry for personal@example.com.
		// ReadFile returns fake content for both allowed_signers and pub key paths.
		ReadFile: func(_ string) ([]byte, error) {
			return []byte(`other@example.com namespaces="git" ssh-ed25519 AAAAC3FakeKey` + "\n"), nil
		},
		RunGitConfigGet: func(_, _ string) (string, error) {
			return "ssh", nil // gpg.format = ssh → is a signing identity
		},
		AllowedSignersPath: "/home/u/.ssh/allowed_signers",
		// AddWiring required so buildSignersFix returns a non-nil Fix.
		AddWiring: func(_, _, _ string) error { return nil },
	}

	findings := checks.CheckCoherence(d)

	var found bool
	for _, f := range findings {
		if f.Severity == doctor.SeverityError && cohContains(f.Title, "no entry for") {
			found = true
			if f.Fix == nil {
				t.Error("allowed_signers missing finding must carry a Fix descriptor")
			}
		}
	}
	if !found {
		t.Errorf("expected 'no entry for' allowed_signers finding, got: %v", cohTitles(findings))
	}
}

// TestCoherenceGPGFormat: fragment gpg.format != "ssh" → error finding (no Fix).
func TestCoherenceGPGFormat(t *testing.T) {
	acct := makeAccount("work", "work.github.com", "work@example.com",
		"/home/u/.ssh/gitid_work", "/home/u/.gitconfig.d/work", "")
	d := doctor.Deps{
		Stat:       cohStat("/home/u/.ssh/gitid_work", "/home/u/.gitconfig.d/work"),
		Identities: []identity.Account{acct},
		ManagedHosts: map[string]sshconfig.SSHHostInfo{
			"work": {Alias: "work.github.com", IdentitiesOnly: true},
		},
		ReadFile: func(_ string) ([]byte, error) {
			return signerLineFor("work@example.com"), nil
		},
		RunGitConfigGet: func(_, key string) (string, error) {
			if key == "gpg.format" {
				return "openpgp", nil // wrong gpg.format
			}
			return "", nil
		},
		AllowedSignersPath: "/home/u/.ssh/allowed_signers",
	}

	findings := checks.CheckCoherence(d)

	var found bool
	for _, f := range findings {
		if f.Severity == doctor.SeverityError && cohContains(f.Title, "gpg.format") {
			found = true
			if f.Fix != nil {
				t.Error("gpg.format mismatch must NOT carry a Fix descriptor (no auto-fix, D-17)")
			}
		}
	}
	if !found {
		t.Errorf("expected gpg.format finding, got: %v", cohTitles(findings))
	}
}

// TestCoherenceEmailMismatch: allowed_signers line email not byte-equal to user.email → error + Fix.
// This is Pitfall 6: must use == not EqualFold.
func TestCoherenceEmailMismatch(t *testing.T) {
	acct := makeAccount("personal", "personal.github.com", "personal@example.com",
		"/home/u/.ssh/gitid_personal", "/home/u/.gitconfig.d/personal", "")
	// The line has a case-differing principal (byte-mismatch).
	mismatchLine := []byte("Personal@Example.com namespaces=\"git\" ssh-ed25519 AAAAC3FakeKey\n")
	d := doctor.Deps{
		Stat:       cohStat("/home/u/.ssh/gitid_personal", "/home/u/.gitconfig.d/personal"),
		Identities: []identity.Account{acct},
		ManagedHosts: map[string]sshconfig.SSHHostInfo{
			"personal": {Alias: "personal.github.com", IdentitiesOnly: true},
		},
		// ReadFile returns the mismatch line for allowed_signers and the same fake
		// bytes for the pub key path (non-empty so buildSignersFix can proceed).
		ReadFile: func(_ string) ([]byte, error) {
			return mismatchLine, nil
		},
		RunGitConfigGet: func(_, _ string) (string, error) {
			return "ssh", nil
		},
		AllowedSignersPath: "/home/u/.ssh/allowed_signers",
		// AddWiring required so buildSignersFix returns a non-nil Fix.
		AddWiring: func(_, _, _ string) error { return nil },
	}

	findings := checks.CheckCoherence(d)

	var found bool
	for _, f := range findings {
		if f.Severity == doctor.SeverityError && cohContains(f.Title, "email mismatch") {
			found = true
			if f.Fix == nil {
				t.Error("email mismatch finding must carry a Fix descriptor")
			}
		}
	}
	if !found {
		t.Errorf("expected 'email mismatch' finding (Pitfall 6 byte-exact check), got: %v", cohTitles(findings))
	}
}

// TestCoherenceIncompleteMapsHere: account.Incomplete != "" surfaces under Coherence, not Orphans.
func TestCoherenceIncompleteMapsHere(t *testing.T) {
	acct := makeAccount("work", "work.github.com", "work@example.com",
		"/home/u/.ssh/gitid_work", "", "fragment-file")
	d := doctor.Deps{
		Stat:       cohStat("/home/u/.ssh/gitid_work"),
		Identities: []identity.Account{acct},
		ManagedHosts: map[string]sshconfig.SSHHostInfo{
			"work": {Alias: "work.github.com", IdentitiesOnly: true},
		},
		ReadFile: func(_ string) ([]byte, error) {
			return nil, errors.New("no allowed_signers path available")
		},
		RunGitConfigGet: func(_, _ string) (string, error) {
			return "ssh", nil
		},
		AllowedSignersPath: "/home/u/.ssh/allowed_signers",
	}

	findings := checks.CheckCoherence(d)

	var coherenceFound bool
	for _, f := range findings {
		if f.Family == doctor.FamilyCoherence {
			coherenceFound = true
		}
		if f.Family == doctor.FamilyOrphans {
			t.Errorf("incomplete account must NOT produce Orphans findings; got: %q", f.Title)
		}
	}
	if !coherenceFound {
		t.Errorf("expected at least one Coherence finding for Incomplete account, got none")
	}
}

// TestCoherenceAllPass: all artifacts resolve, IdentitiesOnly yes, gpg.format ssh,
// correct signers line → zero findings.
func TestCoherenceAllPass(t *testing.T) {
	acct := makeAccount("work", "work.github.com", "work@example.com",
		"/home/u/.ssh/gitid_work", "/home/u/.gitconfig.d/work", "")
	d := doctor.Deps{
		Stat:       cohStat("/home/u/.ssh/gitid_work", "/home/u/.gitconfig.d/work"),
		Identities: []identity.Account{acct},
		ManagedHosts: map[string]sshconfig.SSHHostInfo{
			"work": {Alias: "work.github.com", IdentitiesOnly: true},
		},
		ReadFile: func(_ string) ([]byte, error) {
			return signerLineFor("work@example.com"), nil
		},
		RunGitConfigGet: func(_, _ string) (string, error) {
			return "ssh", nil // gpg.format = ssh
		},
		AllowedSignersPath: "/home/u/.ssh/allowed_signers",
	}

	findings := checks.CheckCoherence(d)

	if len(findings) != 0 {
		t.Errorf("expected zero findings for fully-coherent identity, got %d: %v",
			len(findings), cohTitles(findings))
	}
}

// boolPtr returns a pointer to b — HostBlockFacts.IdentitiesOnly needs a
// *bool so "unset" (nil) is never conflated with "explicitly no" (false).
func boolPtr(b bool) *bool { return &b }

// TestCoherenceHandWrittenIdentitiesOnlyContradiction: a hand-written (no
// ManagedBlockName) Host stanza with explicit IdentitiesOnly no + a non-empty
// IdentityFile produces exactly one error finding, distinct from the
// existing managed-block-only IdentitiesOnly check.
func TestCoherenceHandWrittenIdentitiesOnlyContradiction(t *testing.T) {
	d := doctor.Deps{
		Stat:          cohStat(),
		SSHConfigPath: "/home/u/.ssh/config",
		AllHostBlocks: []sshconfig.HostBlockFacts{
			{
				Pattern:        "clientb.github.com",
				IdentitiesOnly: boolPtr(false),
				IdentityFile:   "~/.ssh/id_ed25519_clientb",
				LineNumber:     5,
				// ManagedBlockName empty — hand-written.
			},
		},
	}

	findings := checks.CheckCoherence(d)

	var found int
	for _, f := range findings {
		if f.Severity == doctor.SeverityError && f.Family == doctor.FamilyCoherence &&
			cohContains(f.Title, "IdentitiesOnly no contradicts") {
			found++
			if f.Target != "SSH" {
				t.Errorf("Target = %q, want SSH", f.Target)
			}
			if f.Rewrite == nil {
				t.Fatal("finding must carry a Rewrite descriptor (D-09 surgical rewrite target)")
			}
			if f.Rewrite.HostPattern != "clientb.github.com" || f.Rewrite.Directive != "IdentitiesOnly" || f.Rewrite.NewValue != "yes" {
				t.Errorf("Rewrite = %+v, want {clientb.github.com IdentitiesOnly yes}", f.Rewrite)
			}
		}
	}
	if found != 1 {
		t.Fatalf("expected exactly 1 hand-written contradiction finding, got %d: %v", found, cohTitles(findings))
	}
}

// TestCoherenceHandWrittenIdentitiesOnly_ManagedBlockSkipped proves the new
// check never double-reports a gitid-managed stanza: coherenceForAccount's
// existing Check 3 already covers that case.
func TestCoherenceHandWrittenIdentitiesOnly_ManagedBlockSkipped(t *testing.T) {
	d := doctor.Deps{
		Stat:          cohStat(),
		SSHConfigPath: "/home/u/.ssh/config",
		AllHostBlocks: []sshconfig.HostBlockFacts{
			{
				Pattern:          "work.github.com",
				IdentitiesOnly:   boolPtr(false),
				IdentityFile:     "~/.ssh/id_ed25519_work",
				ManagedBlockName: "work", // gitid-managed
			},
		},
	}

	findings := checks.CheckCoherence(d)

	for _, f := range findings {
		if cohContains(f.Title, "IdentitiesOnly no contradicts") {
			t.Errorf("the hand-written check must skip a gitid-managed stanza, got: %v", cohTitles(findings))
		}
	}
}

// TestCoherenceHandWrittenIdentitiesOnly_UnsetNeverFlagged proves an unset
// (nil) IdentitiesOnly is never conflated with an explicit "no".
func TestCoherenceHandWrittenIdentitiesOnly_UnsetNeverFlagged(t *testing.T) {
	d := doctor.Deps{
		Stat:          cohStat(),
		SSHConfigPath: "/home/u/.ssh/config",
		AllHostBlocks: []sshconfig.HostBlockFacts{
			{Pattern: "bare.example.com", IdentitiesOnly: nil, IdentityFile: "~/.ssh/id_ed25519_bare"},
		},
	}

	findings := checks.CheckCoherence(d)

	for _, f := range findings {
		if cohContains(f.Title, "IdentitiesOnly no contradicts") {
			t.Errorf("an unset IdentitiesOnly must never be flagged, got: %v", cohTitles(findings))
		}
	}
}

// TestCoherenceHandWrittenIdentitiesOnly_NoIdentityFileNeverFlagged proves a
// hand-written stanza with IdentitiesOnly no but NO IdentityFile is not a
// contradiction (there is nothing to fall back past).
func TestCoherenceHandWrittenIdentitiesOnly_NoIdentityFileNeverFlagged(t *testing.T) {
	d := doctor.Deps{
		Stat:          cohStat(),
		SSHConfigPath: "/home/u/.ssh/config",
		AllHostBlocks: []sshconfig.HostBlockFacts{
			{Pattern: "noidfile.example.com", IdentitiesOnly: boolPtr(false), IdentityFile: ""},
		},
	}

	findings := checks.CheckCoherence(d)

	for _, f := range findings {
		if cohContains(f.Title, "IdentitiesOnly no contradicts") {
			t.Errorf("a stanza with no IdentityFile must never be flagged, got: %v", cohTitles(findings))
		}
	}
}

// cohContains reports whether s contains substr.
func cohContains(s, sub string) bool {
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

// cohTitles extracts titles from a findings slice for test error messages.
func cohTitles(findings []doctor.Finding) []string {
	out := make([]string, len(findings))
	for i, f := range findings {
		out[i] = f.Title
	}
	return out
}

// --- Task 1: shadowed-option check (08-05-PLAN.md) ---

// TestCheckCoherenceShadowed: a nameable shadow finding produces one
// Coherence/SSH warning naming the file:line, Fix nil.
func TestCheckCoherenceShadowed(t *testing.T) {
	d := doctor.Deps{
		Stat: cohStat(),
		GlobalSSHShadowCheck: func() globalssh.ShadowResult {
			return globalssh.ShadowResult{Findings: []globalssh.ShadowFinding{
				{Key: "ForwardAgent", WantValue: "no", GotValue: "yes", ShadowedByFile: "/home/u/.ssh/config", ShadowedByLine: 4},
			}}
		},
	}
	findings := checks.CheckCoherence(d)
	var got *doctor.Finding
	for i := range findings {
		if cohContains(findings[i].Title, "ForwardAgent") {
			got = &findings[i]
		}
	}
	if got == nil {
		t.Fatalf("missing shadowed-option finding, got: %v", cohTitles(findings))
	}
	if got.Severity != doctor.SeverityWarning {
		t.Errorf("Severity = %v, want SeverityWarning", got.Severity)
	}
	if got.Family != doctor.FamilyCoherence || got.Target != "SSH" {
		t.Errorf("Family/Target = %v/%v, want Coherence/SSH", got.Family, got.Target)
	}
	if got.Fix != nil {
		t.Error("Fix should be nil (report-only)")
	}
	if !cohContains(got.SuggestedFix, "/home/u/.ssh/config:4") {
		t.Errorf("SuggestedFix = %q, want it to name the file:line", got.SuggestedFix)
	}
	if cohContains(got.SuggestedFix, "available on the Fixer screen") {
		t.Error("SuggestedFix must not claim Fixer-screen availability when Fix is nil")
	}
}

// TestCheckCoherenceShadowedUnnameable: a shadow finding with no
// ShadowedByFile still produces a finding, honestly worded.
func TestCheckCoherenceShadowedUnnameable(t *testing.T) {
	d := doctor.Deps{
		Stat: cohStat(),
		GlobalSSHShadowCheck: func() globalssh.ShadowResult {
			return globalssh.ShadowResult{Findings: []globalssh.ShadowFinding{
				{Key: "HashKnownHosts", WantValue: "yes", GotValue: "no"},
			}}
		},
	}
	findings := checks.CheckCoherence(d)
	var got *doctor.Finding
	for i := range findings {
		if cohContains(findings[i].Title, "HashKnownHosts") {
			got = &findings[i]
		}
	}
	if got == nil {
		t.Fatal("missing shadowed-option finding")
	}
	if !cohContains(got.SuggestedFix, "source unnameable") {
		t.Errorf("SuggestedFix = %q, want it to admit the source is unnameable", got.SuggestedFix)
	}
}

// TestCheckCoherenceShadowedInconclusive: an inconclusive shadow check
// degrades to no finding, never a false positive.
func TestCheckCoherenceShadowedInconclusive(t *testing.T) {
	d := doctor.Deps{
		Stat: cohStat(),
		GlobalSSHShadowCheck: func() globalssh.ShadowResult {
			return globalssh.ShadowResult{Inconclusive: true, Reason: "probe timed out"}
		},
	}
	findings := checks.CheckCoherence(d)
	for _, f := range findings {
		if f.Family == doctor.FamilyCoherence && cohContains(f.Title, "shadowed") {
			t.Errorf("inconclusive shadow check must never produce a finding, got: %v", cohTitles(findings))
		}
	}
}

// TestCheckCoherenceShadowedNilDeps: a nil GlobalSSHShadowCheck produces no
// findings and no panic.
func TestCheckCoherenceShadowedNilDeps(t *testing.T) {
	d := doctor.Deps{Stat: cohStat()}
	findings := checks.CheckCoherence(d)
	if len(findings) != 0 {
		t.Errorf("nil GlobalSSHShadowCheck: got %d findings, want 0", len(findings))
	}
}

// --- Task 1: directive-above-managed-block check ---

// TestCheckCoherenceDirectiveAbove: a hand-written stanza before gitid's
// first managed block produces one Coherence/SSH warning, Fix nil.
func TestCheckCoherenceDirectiveAbove(t *testing.T) {
	d := doctor.Deps{
		Stat: cohStat(),
		AllHostBlocks: []sshconfig.HostBlockFacts{
			{Pattern: "handwritten.example.com", ManagedBlockName: ""},
			{Pattern: "work", ManagedBlockName: "work"},
		},
	}
	findings := checks.CheckCoherence(d)
	var got *doctor.Finding
	for i := range findings {
		if cohContains(findings[i].Title, "handwritten.example.com") {
			got = &findings[i]
		}
	}
	if got == nil {
		t.Fatalf("missing directive-above-block finding, got: %v", cohTitles(findings))
	}
	if got.Severity != doctor.SeverityWarning || got.Family != doctor.FamilyCoherence || got.Target != "SSH" {
		t.Errorf("got Severity=%v Family=%v Target=%v, want Warning/Coherence/SSH", got.Severity, got.Family, got.Target)
	}
	if got.Fix != nil {
		t.Error("Fix should be nil (report-only, D-09)")
	}
}

// TestCheckCoherenceDirectiveAbove_AfterManagedNeverFlagged: a hand-written
// stanza AFTER the first managed block is not "above" it and must not be
// flagged.
func TestCheckCoherenceDirectiveAbove_AfterManagedNeverFlagged(t *testing.T) {
	d := doctor.Deps{
		Stat: cohStat(),
		AllHostBlocks: []sshconfig.HostBlockFacts{
			{Pattern: "work", ManagedBlockName: "work"},
			{Pattern: "handwritten.example.com", ManagedBlockName: ""},
		},
	}
	findings := checks.CheckCoherence(d)
	for _, f := range findings {
		if cohContains(f.Title, "handwritten.example.com") {
			t.Errorf("a hand-written stanza after a managed block must not be flagged, got: %v", cohTitles(findings))
		}
	}
}

// TestCheckCoherenceDirectiveAbove_AllManagedNeverFlagged: an all-managed
// config (no hand-written stanzas at all) produces no directive-above
// findings.
func TestCheckCoherenceDirectiveAbove_AllManagedNeverFlagged(t *testing.T) {
	d := doctor.Deps{
		Stat: cohStat(),
		AllHostBlocks: []sshconfig.HostBlockFacts{
			{Pattern: "work", ManagedBlockName: "work"},
			{Pattern: "*", ManagedBlockName: "global-ssh"},
		},
	}
	findings := checks.CheckCoherence(d)
	for _, f := range findings {
		if f.Family == doctor.FamilyCoherence && cohContains(f.Title, "precedes gitid's managed") {
			t.Errorf("an all-managed config must never produce a directive-above finding, got: %v", cohTitles(findings))
		}
	}
}

// --- Task 2: author-resolution check ---

// TestCheckCoherenceAuthorResolution: a mismatched matched-directory
// resolution produces one Coherence/Git error, scoped to the identity.
func TestCheckCoherenceAuthorResolution(t *testing.T) {
	d := doctor.Deps{
		Stat:       cohStat(),
		Identities: []identity.Account{{Name: "work", GitName: "Work Name", GitEmail: "work@example.com"}},
		AuthorResolutionCheck: func(identityName string) (globalgit.AuthorResolution, bool, error) {
			if identityName != "work" {
				return globalgit.AuthorResolution{}, false, nil
			}
			return globalgit.AuthorResolution{
				MatchedOutcome: globalgit.MatchedVerified,
				Matched: globalgit.DirectoryResolution{
					Name:  globalgit.AuthorKeyResolution{Value: "Wrong Name"},
					Email: globalgit.AuthorKeyResolution{Value: "work@example.com"},
				},
			}, true, nil
		},
	}
	findings := checks.CheckCoherence(d)
	var got *doctor.Finding
	for i := range findings {
		if cohContains(findings[i].Title, "author resolution does not match") {
			got = &findings[i]
		}
	}
	if got == nil {
		t.Fatalf("missing author-resolution finding, got: %v", cohTitles(findings))
	}
	if got.Severity != doctor.SeverityError || got.Family != doctor.FamilyCoherence || got.Target != "Git" {
		t.Errorf("got Severity=%v Family=%v Target=%v, want Error/Coherence/Git", got.Severity, got.Family, got.Target)
	}
	if got.IdentityName != "work" {
		t.Errorf("IdentityName = %q, want %q", got.IdentityName, "work")
	}
	if got.Fix != nil {
		t.Error("Fix should be nil (report-only)")
	}
}

// TestCheckCoherenceAuthorResolution_HealthyNeverFlagged: a matching
// resolution produces no finding for that identity.
func TestCheckCoherenceAuthorResolution_HealthyNeverFlagged(t *testing.T) {
	d := doctor.Deps{
		Stat:       cohStat(),
		Identities: []identity.Account{{Name: "work", GitName: "Work Name", GitEmail: "work@example.com"}},
		AuthorResolutionCheck: func(string) (globalgit.AuthorResolution, bool, error) {
			return globalgit.AuthorResolution{
				MatchedOutcome: globalgit.MatchedVerified,
				Matched: globalgit.DirectoryResolution{
					Name:  globalgit.AuthorKeyResolution{Value: "Work Name"},
					Email: globalgit.AuthorKeyResolution{Value: "work@example.com"},
				},
			}, true, nil
		},
	}
	findings := checks.CheckCoherence(d)
	for _, f := range findings {
		if cohContains(f.Title, "author resolution does not match") {
			t.Errorf("a matching resolution must never be flagged, got: %v", cohTitles(findings))
		}
	}
}

// TestCheckCoherenceAuthorResolution_NotVerifiableNeverFlagged: ok=false
// (MatchedNotVerifiable, or no includeIf on record) is a graceful
// no-finding state, never a false positive.
func TestCheckCoherenceAuthorResolution_NotVerifiableNeverFlagged(t *testing.T) {
	d := doctor.Deps{
		Stat:       cohStat(),
		Identities: []identity.Account{{Name: "work", GitName: "Work Name", GitEmail: "work@example.com"}},
		AuthorResolutionCheck: func(string) (globalgit.AuthorResolution, bool, error) {
			return globalgit.AuthorResolution{}, false, nil
		},
	}
	findings := checks.CheckCoherence(d)
	for _, f := range findings {
		if cohContains(f.Title, "author resolution does not match") {
			t.Errorf("MatchedNotVerifiable must never be flagged, got: %v", cohTitles(findings))
		}
	}
}
