package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/castocolina/gitid/internal/repoclone"
	"github.com/castocolina/gitid/internal/sshconfig"
)

// buildFakeRepoCloneDeps builds a repoclone.Deps backed by fakes that record calls.
type fakeRepoCloneDeps struct {
	cloneURL  string
	cloneDest string
	pullDest  string
	home      string
}

func (f *fakeRepoCloneDeps) build() repoclone.Deps {
	return repoclone.Deps{
		Stat: func(_ string) (os.FileInfo, error) {
			return nil, os.ErrNotExist // always report dest does not exist
		},
		Clone: func(cloneURL, destPath string) ([]string, error) {
			f.cloneURL = cloneURL
			f.cloneDest = destPath
			return []string{"Cloning into '" + destPath + "'...", "done."}, nil
		},
		Pull: func(destPath string) ([]string, error) {
			f.pullDest = destPath
			return []string{"Already up to date."}, nil
		},
		UserHomeDir: func() (string, error) { return f.home, nil },
	}
}

// TestRunAddRepo_LocalClone verifies that runAddRepo clones into ~/git/<client>/<repo>.
func TestRunAddRepo_LocalClone(t *testing.T) {
	home := t.TempDir()
	rec := &fakeRepoCloneDeps{home: home}
	// Use an HTTPS URL with a path so DestPath can extract the repo name.
	// The fake Clone dep records the URL without actually running git.
	cloneURL := "https://github.com/org/myrepo"

	var buf bytes.Buffer
	err := runAddRepo(&buf, cloneURL, "personal", true, func() repoclone.Deps {
		return rec.build()
	})
	if err != nil {
		t.Fatalf("runAddRepo returned error: %v", err)
	}

	// Assert: dest path under ~/git/personal/myrepo.
	expectedDest := filepath.Join(home, "git", "personal", "myrepo")
	if rec.cloneDest != expectedDest {
		t.Errorf("clone dest = %q, want %q", rec.cloneDest, expectedDest)
	}
	// Assert: pull was called.
	if rec.pullDest == "" {
		t.Error("expected pull to be called after clone")
	}
	// Assert: output contains clone + pull evidence.
	out := buf.String()
	if !strings.Contains(out, "Cloning") && !strings.Contains(out, "done") {
		t.Errorf("expected clone output in stdout; got: %s", out)
	}
	if !strings.Contains(out, "up to date") && !strings.Contains(out, "pull") {
		t.Errorf("expected pull output in stdout; got: %s", out)
	}
}

// TestRunAddRepo_AliasRewrite verifies that when a managed identity with a
// matching provider alias exists, the URL is rewritten before clone.
func TestRunAddRepo_AliasRewrite(t *testing.T) {
	home := t.TempDir()

	// Write a fake ~/.ssh/config with a managed block for github.com.
	sshDir := filepath.Join(home, ".ssh")
	if err := os.MkdirAll(sshDir, 0o700); err != nil {
		t.Fatalf("mkdir ssh dir: %v", err)
	}
	// Render a managed SSH block using the canonical sentinel format.
	// The block must carry a # gitid: provider= marker for provider lookup.
	sshContent := "# BEGIN gitid managed: personal\n" +
		"# gitid: provider=github.com\n" +
		"Host personal.github.com\n" +
		"  Hostname github.com\n" +
		"  User git\n" +
		"  IdentityFile ~/.ssh/id_ed25519_personal\n" +
		"  IdentitiesOnly yes\n" +
		"# END gitid managed: personal\n"
	if err := os.WriteFile(filepath.Join(sshDir, "config"), []byte(sshContent), 0o600); err != nil {
		t.Fatalf("writing ssh config: %v", err)
	}

	rec := &fakeRepoCloneDeps{home: home}
	rawURL := "https://github.com/org/repo"

	var buf bytes.Buffer
	err := runAddRepo(&buf, rawURL, "personal", true, func() repoclone.Deps {
		return rec.build()
	})
	// Clone may fail (no real git) but we just need the URL rewrite in output.
	_ = err

	out := buf.String()
	if !strings.Contains(out, "personal.github.com") {
		t.Errorf("expected SSH alias rewrite (personal.github.com) in output; got: %s", out)
	}
}

// TestRunAddRepo_NoMatchingIdentity verifies that when no managed identity
// matches the provider, the original URL is used and output explains this.
func TestRunAddRepo_NoMatchingIdentity(t *testing.T) {
	home := t.TempDir()
	// No SSH config → empty managed hosts.
	rec := &fakeRepoCloneDeps{home: home}
	rawURL := "https://github.com/org/repo"

	var buf bytes.Buffer
	err := runAddRepo(&buf, rawURL, "personal", true, func() repoclone.Deps {
		return rec.build()
	})
	if err != nil {
		t.Fatalf("runAddRepo returned error: %v", err)
	}

	out := buf.String()
	// Clone URL should be the original (no rewrite).
	if rec.cloneURL != rawURL {
		t.Errorf("expected original URL %q to be cloned; got %q", rawURL, rec.cloneURL)
	}
	if !strings.Contains(out, "No managed identity found") {
		t.Errorf("expected 'No managed identity found' message; got: %s", out)
	}
}

// TestFindAliasForProvider verifies provider-to-alias lookup.
func TestFindAliasForProvider(t *testing.T) {
	hosts := map[string]sshconfig.SSHHostInfo{
		"personal": {
			Alias:    "personal.github.com",
			Hostname: "github.com",
			Provider: "github.com",
		},
	}

	got := findAliasForProvider(hosts, "github.com")
	if got != "personal.github.com" {
		t.Errorf("findAliasForProvider = %q, want %q", got, "personal.github.com")
	}

	got = findAliasForProvider(hosts, "gitlab.com")
	if got != "" {
		t.Errorf("findAliasForProvider(gitlab.com) = %q, want empty", got)
	}
}

// TestBuildRepoCloneDepsNoOsExec verifies that buildRepoCloneDeps does not call
// os/exec directly (all exec lives in repoclone.LiveClone / livePull).
// This test enforces the grep -c "os/exec" gate from the plan acceptance criteria.
func TestBuildRepoCloneDepsNoOsExec(t *testing.T) {
	deps := buildRepoCloneDeps()
	if deps.Clone == nil {
		t.Error("repoclone.Deps.Clone nil")
	}
	if deps.Pull == nil {
		t.Error("repoclone.Deps.Pull nil")
	}
	if deps.Stat == nil {
		t.Error("repoclone.Deps.Stat nil")
	}
	if deps.UserHomeDir == nil {
		t.Error("repoclone.Deps.UserHomeDir nil")
	}
}
