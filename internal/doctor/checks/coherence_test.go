package checks_test

import (
	"errors"
	"os"
	"testing"
	"time"

	"github.com/castocolina/gitid/internal/doctor"
	"github.com/castocolina/gitid/internal/doctor/checks"
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
		Stat:       cohStat("/home/u/.ssh/gitid_work", "/home/u/.gitconfig.d/work"),
		Identities: []identity.Account{acct},
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
		ReadFile: func(_ string) ([]byte, error) {
			return []byte(`other@example.com namespaces="git" ssh-ed25519 AAAAC3FakeKey` + "\n"), nil
		},
		RunGitConfigGet: func(_, _ string) (string, error) {
			return "ssh", nil // gpg.format = ssh → is a signing identity
		},
		AllowedSignersPath: "/home/u/.ssh/allowed_signers",
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
		ReadFile: func(_ string) ([]byte, error) {
			return mismatchLine, nil
		},
		RunGitConfigGet: func(_, _ string) (string, error) {
			return "ssh", nil
		},
		AllowedSignersPath: "/home/u/.ssh/allowed_signers",
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
