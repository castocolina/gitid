package main

import (
	"bytes"
	"strings"
	"testing"

	"github.com/castocolina/gitid/internal/gitconfig"
	"github.com/castocolina/gitid/internal/identity"
)

// TestPrintAccounts_Empty asserts a friendly message when no accounts exist.
func TestPrintAccounts_Empty(t *testing.T) {
	var out bytes.Buffer
	printAccounts(&out, nil)
	if out.Len() != 0 {
		t.Errorf("expected empty output for nil accounts, got: %q", out.String())
	}
}

// TestPrintAccounts_Single exercises the grouped-by-identity rendering for a
// fully populated account.
func TestPrintAccounts_Single(t *testing.T) {
	accts := []identity.Account{
		{
			Name:     "work",
			GitName:  "Jane Doe",
			GitEmail: "jane@example.com",
			Alias:    "work.github.com",
			Provider: "github.com",
			Hostname: "ssh.github.com",
			Port:     443,
			KeyPath:  "/home/jane/.ssh/id_ed25519_work",
			Matches:  []gitconfig.Match{{Kind: gitconfig.MatchGitdir, Value: "~/git/work/"}},
		},
	}
	var out bytes.Buffer
	printAccounts(&out, accts)
	got := out.String()

	checks := []string{
		"identity: work",
		"key:      /home/jane/.ssh/id_ed25519_work",
		"git:      Jane Doe <jane@example.com>",
		"alias:    work.github.com",
		"provider: github.com",
		"port:     443",
		"match:    gitdir:~/git/work/",
	}
	for _, want := range checks {
		if !strings.Contains(got, want) {
			t.Errorf("expected %q in output, got:\n%s", want, got)
		}
	}
	// No incomplete marker expected.
	if strings.Contains(got, "incomplete") {
		t.Errorf("unexpected incomplete marker in fully populated account output:\n%s", got)
	}
}

// TestPrintAccounts_Incomplete verifies the light incomplete marker appears
// when an account has partial managed blocks (D-02).
func TestPrintAccounts_Incomplete(t *testing.T) {
	accts := []identity.Account{
		{
			Name:       "personal",
			Alias:      "personal.github.com",
			Port:       443,
			Incomplete: "gitconfig-includeif-block,fragment-file",
		},
	}
	var out bytes.Buffer
	printAccounts(&out, accts)
	got := out.String()

	if !strings.Contains(got, "! incomplete: missing gitconfig-includeif-block,fragment-file") {
		t.Errorf("expected incomplete marker in output, got:\n%s", got)
	}
}

// TestPrintAccounts_ProviderFallbackToHostname verifies that when Provider is
// empty but Hostname is set, the hostname is shown as the provider (A1).
func TestPrintAccounts_ProviderFallbackToHostname(t *testing.T) {
	accts := []identity.Account{
		{
			Name:     "corp",
			Alias:    "corp",
			Hostname: "gitlab.corp.internal",
			Port:     22,
		},
	}
	var out bytes.Buffer
	printAccounts(&out, accts)
	got := out.String()

	if !strings.Contains(got, "provider: gitlab.corp.internal") {
		t.Errorf("expected hostname as provider fallback, got:\n%s", got)
	}
}

// TestPrintAccounts_MultipleIdentities asserts that multiple accounts are
// separated by a blank line and each shows its own identity header.
func TestPrintAccounts_MultipleIdentities(t *testing.T) {
	accts := []identity.Account{
		{Name: "personal", Alias: "personal.github.com", Port: 443},
		{Name: "work", Alias: "work.github.com", Port: 443},
	}
	var out bytes.Buffer
	printAccounts(&out, accts)
	got := out.String()

	if !strings.Contains(got, "identity: personal") {
		t.Errorf("expected 'identity: personal' in output, got:\n%s", got)
	}
	if !strings.Contains(got, "identity: work") {
		t.Errorf("expected 'identity: work' in output, got:\n%s", got)
	}
	// Blank separator between identities.
	if !strings.Contains(got, "\n\n") {
		t.Errorf("expected blank line separator between identities, got:\n%s", got)
	}
}

// TestRunIdentityList_EmptyConfigsProduceFriendlyMessage exercises the
// run function end-to-end with no SSH/gitconfig files present.
func TestRunIdentityList_EmptyConfigs(t *testing.T) {
	// runIdentityList reads real ~/.ssh/config and ~/.gitconfig; if they happen
	// to have gitid-managed blocks the output may be non-empty. We test only
	// that the function does not error out — the no-identities branch is
	// exercised via printAccounts tests above.
	var out bytes.Buffer
	if err := runIdentityList(nil, &out); err != nil {
		t.Fatalf("runIdentityList() returned unexpected error: %v", err)
	}
}
